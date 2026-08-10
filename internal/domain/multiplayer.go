package domain

import (
	"fmt"
	"strings"
	"time"
)

// PlayerSlot binds a player (by id — e.g. a Telegram user id) to the party
// characters they control (a player may control several — issue #29), the active
// one used when an action/line doesn't name a character, and a display name.
//
// CharacterName is the legacy single-character field; sessions saved before #29
// carry it and are migrated into Characters/Active on load (see
// ensureInitialized → migratePlayerSlots). New code uses Characters/Active.
type PlayerSlot struct {
	DisplayName   string   `json:"display_name"`
	CharacterName string   `json:"character_name,omitempty"` // legacy (migrated)
	Characters    []string `json:"characters,omitempty"`
	Active        string   `json:"active,omitempty"`
}

// controls reports whether the slot controls a character (case-insensitive).
func (p *PlayerSlot) controls(name string) bool {
	for _, c := range p.Characters {
		if strings.EqualFold(c, name) {
			return true
		}
	}
	return false
}

// add appends a controlled character (if new) and makes it active.
func (p *PlayerSlot) add(name string) {
	if !p.controls(name) {
		p.Characters = append(p.Characters, name)
	}
	p.Active = name
}

// remove drops a controlled character, fixing up the active pointer; reports
// whether any characters remain.
func (p *PlayerSlot) remove(name string) (remaining bool) {
	out := p.Characters[:0]
	for _, c := range p.Characters {
		if !strings.EqualFold(c, name) {
			out = append(out, c)
		}
	}
	p.Characters = out
	if strings.EqualFold(p.Active, name) {
		p.Active = ""
		if len(p.Characters) > 0 {
			p.Active = p.Characters[0]
		}
	}
	return len(p.Characters) > 0
}

// migratePlayerSlots upgrades legacy single-character slots to the multi-character
// model. Caller holds s.mu (invoked from ensureInitialized).
func (s *SessionState) migratePlayerSlots() {
	for _, slot := range s.Players {
		if slot == nil {
			continue
		}
		if len(slot.Characters) == 0 && slot.CharacterName != "" {
			slot.Characters = []string{slot.CharacterName}
		}
		if slot.Active == "" && len(slot.Characters) > 0 {
			slot.Active = slot.Characters[0]
		}
		slot.CharacterName = "" // fully migrated; Characters/Active are authoritative
	}
}

// RoundAction is one player's declared action for the current turn.
type RoundAction struct {
	PlayerID      string    `json:"player_id"`
	DisplayName   string    `json:"display_name"`
	CharacterName string    `json:"character_name"`
	Text          string    `json:"text"`
	At            time.Time `json:"at"`
}

// TurnRound buffers the players' declared actions until the DM resolves them.
type TurnRound struct {
	Actions []RoundAction `json:"actions"`
}

// controlledByOther reports the display name of a DIFFERENT player controlling
// the character, or "". Caller holds s.mu.
func (s *SessionState) controlledByOther(playerID, charName string) string {
	for pid, slot := range s.Players {
		if pid != playerID && slot.controls(charName) {
			return slot.DisplayName
		}
	}
	return ""
}

// ClaimCharacter gives the named party member to a player, ADDING it to any
// characters they already control (issue #29) and making it their active
// character. It fails if the character isn't in the party or is already
// controlled by a different player; re-claiming one the player already controls
// just makes it active. Returns the canonical character name.
func (s *SessionState) ClaimCharacter(playerID, displayName, charName string) (string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if strings.TrimSpace(charName) == "" {
		return "", fmt.Errorf("name a character to play")
	}
	c := s.resolveCharacter(charName)
	if c == nil {
		return "", fmt.Errorf("no character %q in the party", charName)
	}
	if who := s.controlledByOther(playerID, c.Name); who != "" {
		return "", fmt.Errorf("%s is already controlled by %s", c.Name, who)
	}
	if s.Players == nil {
		s.Players = make(map[string]*PlayerSlot)
	}
	slot := s.Players[playerID]
	if slot == nil {
		slot = &PlayerSlot{DisplayName: displayName}
		s.Players[playerID] = slot
	}
	slot.DisplayName = displayName
	slot.add(c.Name)
	s.record(LogEntry{Type: LogParty, Message: fmt.Sprintf("%s now plays %s", displayName, c.Name),
		Data: map[string]any{"player": playerID, "character": c.Name}})
	s.touch()
	return c.Name, nil
}

// SetActiveCharacter chooses which of a player's controlled characters is active
// (the default target for /do and /chat). Returns the canonical name.
func (s *SessionState) SetActiveCharacter(playerID, charName string) (string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	slot := s.Players[playerID]
	if slot == nil || len(slot.Characters) == 0 {
		return "", fmt.Errorf("pick a character first")
	}
	for _, c := range slot.Characters {
		if strings.EqualFold(c, strings.TrimSpace(charName)) {
			slot.Active = c
			s.touch()
			return c, nil
		}
	}
	return "", fmt.Errorf("you don't control %q; you play: %s", charName, strings.Join(slot.Characters, ", "))
}

// ReleaseCharacter unassigns ALL of a player's characters (and drops their
// pending actions).
func (s *SessionState) ReleaseCharacter(playerID string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.Players, playerID)
	if s.Round != nil {
		s.Round.Actions = filterActions(s.Round.Actions, playerID)
	}
	s.touch()
}

// PlayerCharacterName returns a player's ACTIVE character (empty if none). Kept
// for callers that operate on "the player's character" (e.g. sheet edits).
func (s *SessionState) PlayerCharacterName(playerID string) string {
	s.mu.Lock()
	defer s.mu.Unlock()
	if slot, ok := s.Players[playerID]; ok {
		return slot.Active
	}
	return ""
}

// PlayerCharacterNames returns all characters a player controls (nil if none).
func (s *SessionState) PlayerCharacterNames(playerID string) []string {
	s.mu.Lock()
	defer s.mu.Unlock()
	if slot, ok := s.Players[playerID]; ok {
		return append([]string(nil), slot.Characters...)
	}
	return nil
}

// resolvePlayerCharacter picks which of a player's controlled characters an
// action/line targets: the named one (case-insensitive) if given and controlled,
// else the active one when no name is given. Caller holds s.mu.
func (s *SessionState) resolvePlayerCharacter(playerID, charName string) (string, error) {
	slot := s.Players[playerID]
	if slot == nil || len(slot.Characters) == 0 {
		return "", fmt.Errorf("pick a character first")
	}
	charName = strings.TrimSpace(charName)
	if charName == "" {
		if slot.Active != "" {
			return slot.Active, nil
		}
		return slot.Characters[0], nil
	}
	for _, c := range slot.Characters {
		if strings.EqualFold(c, charName) {
			return c, nil
		}
	}
	return "", fmt.Errorf("you don't control %q; you play: %s", charName, strings.Join(slot.Characters, ", "))
}

// SubmitAction records a player's action for one of their characters this round.
// charName selects which controlled character acts (empty = the active one); a
// player controlling several can call it once per character. It replaces any
// earlier action for the SAME character this round, so each character has at most
// one pending action.
func (s *SessionState) SubmitAction(playerID, charName, text string) (RoundAction, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	slot, ok := s.Players[playerID]
	if !ok {
		return RoundAction{}, fmt.Errorf("pick a character first")
	}
	if strings.TrimSpace(text) == "" {
		return RoundAction{}, fmt.Errorf("empty action")
	}
	target, err := s.resolvePlayerCharacter(playerID, charName)
	if err != nil {
		return RoundAction{}, err
	}
	if s.Round == nil {
		s.Round = &TurnRound{}
	}
	act := RoundAction{
		PlayerID:      playerID,
		DisplayName:   slot.DisplayName,
		CharacterName: target,
		Text:          strings.TrimSpace(text),
		At:            time.Now(),
	}
	// Replace an earlier action for the same character this round.
	for i := range s.Round.Actions {
		if s.Round.Actions[i].PlayerID == playerID && strings.EqualFold(s.Round.Actions[i].CharacterName, target) {
			s.Round.Actions[i] = act
			s.touch()
			return act, nil
		}
	}
	s.Round.Actions = append(s.Round.Actions, act)
	s.touch()
	return act, nil
}

// ResolvePlayerCharacter resolves which controlled character an action/line
// targets (named or active), under the lock. Used by the /chat command to
// attribute in-character dialogue to the right PC.
func (s *SessionState) ResolvePlayerCharacter(playerID, charName string) (string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.resolvePlayerCharacter(playerID, charName)
}

// RoundActions returns a copy of the current round's declared actions.
func (s *SessionState) RoundActions() []RoundAction {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.Round == nil {
		return nil
	}
	out := make([]RoundAction, len(s.Round.Actions))
	copy(out, s.Round.Actions)
	return out
}

// ResetRound clears the current round's buffered actions.
func (s *SessionState) ResetRound() {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.Round != nil {
		s.Round.Actions = nil
	}
	s.touch()
}

// RemoveResolvedActions drops exactly the given actions (matched by player and
// timestamp) from the round buffer, leaving any actions submitted after they were
// snapshotted — e.g. while the DM was resolving the turn — so those aren't lost.
func (s *SessionState) RemoveResolvedActions(resolved []RoundAction) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.Round == nil || len(resolved) == 0 {
		return
	}
	drop := make(map[string]bool, len(resolved))
	for _, a := range resolved {
		drop[a.PlayerID+"|"+a.At.Format(time.RFC3339Nano)] = true
	}
	kept := s.Round.Actions[:0]
	for _, a := range s.Round.Actions {
		if !drop[a.PlayerID+"|"+a.At.Format(time.RFC3339Nano)] {
			kept = append(kept, a)
		}
	}
	s.Round.Actions = kept
	s.touch()
}

// PendingPlayers lists the controlled CHARACTERS that have not yet declared an
// action this round (as "Character (player)"), so a player controlling several
// characters is expected to act for each. A round is "complete" when this is
// empty.
func (s *SessionState) PendingPlayers() []string {
	s.mu.Lock()
	defer s.mu.Unlock()
	acted := map[string]bool{} // playerID|character
	if s.Round != nil {
		for _, a := range s.Round.Actions {
			acted[a.PlayerID+"|"+strings.ToLower(a.CharacterName)] = true
		}
	}
	var pending []string
	for pid, slot := range s.Players {
		for _, c := range slot.Characters {
			if !acted[pid+"|"+strings.ToLower(c)] {
				label := c
				if slot.DisplayName != "" {
					label += " (" + slot.DisplayName + ")"
				}
				pending = append(pending, label)
			}
		}
	}
	return pending
}

// AssignByUsername reserves a party character for a Telegram @username that hasn't
// picked yet; the binding takes effect when that user next sends a message (see
// ResolvePending). Fails if the character is unknown, already controlled, or
// already reserved for a different username. Returns the canonical name.
func (s *SessionState) AssignByUsername(username, charName string) (string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	username = strings.ToLower(strings.TrimPrefix(strings.TrimSpace(username), "@"))
	if username == "" {
		return "", fmt.Errorf("give a @username")
	}
	c := s.resolveCharacter(charName)
	if c == nil {
		return "", fmt.Errorf("no character %q in the party", charName)
	}
	if who := s.controlledByOther("", c.Name); who != "" {
		return "", fmt.Errorf("%s is already controlled by %s", c.Name, who)
	}
	for u, pc := range s.PendingAssignments {
		if u != username && strings.EqualFold(pc, c.Name) {
			return "", fmt.Errorf("%s is already reserved for @%s", c.Name, u)
		}
	}
	if s.PendingAssignments == nil {
		s.PendingAssignments = make(map[string]string)
	}
	s.PendingAssignments[username] = c.Name
	s.record(LogEntry{Type: LogParty, Message: fmt.Sprintf("Assigned %s to @%s", c.Name, username)})
	s.touch()
	return c.Name, nil
}

// ResolvePending binds a pending @username assignment to a real player the first
// time they appear. Returns the character name and whether a binding happened.
func (s *SessionState) ResolvePending(playerID, username, display string) (string, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	username = strings.ToLower(strings.TrimSpace(username))
	if username == "" || len(s.PendingAssignments) == 0 {
		return "", false
	}
	pc, ok := s.PendingAssignments[username]
	if !ok {
		return "", false
	}
	delete(s.PendingAssignments, username)
	c := s.resolveCharacter(pc)
	if c == nil {
		return "", false
	}
	if who := s.controlledByOther(playerID, c.Name); who != "" {
		return "", false // taken in the meantime
	}
	if s.Players == nil {
		s.Players = make(map[string]*PlayerSlot)
	}
	slot := s.Players[playerID]
	if slot == nil {
		slot = &PlayerSlot{DisplayName: display}
		s.Players[playerID] = slot
	}
	if slot.DisplayName == "" {
		slot.DisplayName = display
	}
	slot.add(c.Name) // a player may already control other characters (#29)
	s.record(LogEntry{Type: LogParty, Message: fmt.Sprintf("%s now plays %s (assigned)", display, c.Name)})
	s.touch()
	return c.Name, true
}

// PendingByCharacter returns character name → reserved @username, for display.
func (s *SessionState) PendingByCharacter() map[string]string {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make(map[string]string, len(s.PendingAssignments))
	for u, pc := range s.PendingAssignments {
		out[pc] = u
	}
	return out
}

// Controllers returns a map of character name → controlling player's display
// name, for showing who plays whom.
func (s *SessionState) Controllers() map[string]string {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make(map[string]string, len(s.Players))
	for _, slot := range s.Players {
		for _, c := range slot.Characters {
			out[c] = slot.DisplayName
		}
	}
	return out
}

// GameStarted reports whether the game has begun.
func (s *SessionState) GameStarted() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.Started
}

// StartGame marks the game as begun and returns whether it did so now (false if it
// was already started).
func (s *SessionState) StartGame() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.Started {
		return false
	}
	s.Started = true
	s.record(LogEntry{Type: LogSystem, Message: "Game started"})
	s.touch()
	return true
}

// MarkStartedIfInProgress marks the game started when there's evidence it is
// already underway — the DM has narrated (an assistant message) or a round is
// buffered — so a session played in the GUI or saved before this feature isn't
// treated as fresh (which would demand /begin and re-narrate an opening).
func (s *SessionState) MarkStartedIfInProgress() {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.Started {
		return
	}
	inProgress := s.Round != nil && len(s.Round.Actions) > 0
	if s.Conversation != nil {
		for _, m := range s.Conversation.Messages {
			if m.Role == RoleAssistant {
				inProgress = true
				break
			}
		}
	}
	if inProgress {
		s.Started = true
		s.touch()
	}
}

// PlayerCount returns how many players currently control a character.
func (s *SessionState) PlayerCount() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return len(s.Players)
}

func filterActions(actions []RoundAction, dropPlayerID string) []RoundAction {
	out := actions[:0]
	for _, a := range actions {
		if a.PlayerID != dropPlayerID {
			out = append(out, a)
		}
	}
	return out
}
