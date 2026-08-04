package domain

import (
	"fmt"
	"strings"
	"time"
)

// PlayerSlot binds a player (by id — e.g. a Telegram user id) to the party
// character they control and a human-readable display name.
type PlayerSlot struct {
	DisplayName   string `json:"display_name"`
	CharacterName string `json:"character_name"`
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

// ClaimCharacter assigns the named party member to a player. It fails if the
// character isn't in the party or is already controlled by a different player;
// re-claiming the same character for the same player is a no-op. Returns the
// canonical character name.
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
	for pid, slot := range s.Players {
		if pid != playerID && strings.EqualFold(slot.CharacterName, c.Name) {
			return "", fmt.Errorf("%s is already controlled by %s", c.Name, slot.DisplayName)
		}
	}
	if s.Players == nil {
		s.Players = make(map[string]*PlayerSlot)
	}
	// Switching to a different character invalidates this player's pending action
	// (it was recorded under the old character), so drop it.
	if old, ok := s.Players[playerID]; ok && !strings.EqualFold(old.CharacterName, c.Name) && s.Round != nil {
		s.Round.Actions = filterActions(s.Round.Actions, playerID)
	}
	s.Players[playerID] = &PlayerSlot{DisplayName: displayName, CharacterName: c.Name}
	s.record(LogEntry{Type: LogParty, Message: fmt.Sprintf("%s now plays %s", displayName, c.Name),
		Data: map[string]any{"player": playerID, "character": c.Name}})
	s.touch()
	return c.Name, nil
}

// ReleaseCharacter unassigns a player's character (and drops any pending action).
func (s *SessionState) ReleaseCharacter(playerID string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.Players, playerID)
	if s.Round != nil {
		s.Round.Actions = filterActions(s.Round.Actions, playerID)
	}
	s.touch()
}

// PlayerCharacterName returns the character a player controls (empty if none).
func (s *SessionState) PlayerCharacterName(playerID string) string {
	s.mu.Lock()
	defer s.mu.Unlock()
	if slot, ok := s.Players[playerID]; ok {
		return slot.CharacterName
	}
	return ""
}

// SubmitAction records a player's action for the current round, replacing any
// earlier action they submitted this round. The player must control a character.
func (s *SessionState) SubmitAction(playerID, text string) (RoundAction, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	slot, ok := s.Players[playerID]
	if !ok {
		return RoundAction{}, fmt.Errorf("pick a character first")
	}
	if strings.TrimSpace(text) == "" {
		return RoundAction{}, fmt.Errorf("empty action")
	}
	if s.Round == nil {
		s.Round = &TurnRound{}
	}
	act := RoundAction{
		PlayerID:      playerID,
		DisplayName:   slot.DisplayName,
		CharacterName: slot.CharacterName,
		Text:          strings.TrimSpace(text),
		At:            time.Now(),
	}
	// Replace an earlier action from the same player this round.
	for i := range s.Round.Actions {
		if s.Round.Actions[i].PlayerID == playerID {
			s.Round.Actions[i] = act
			s.touch()
			return act, nil
		}
	}
	s.Round.Actions = append(s.Round.Actions, act)
	s.touch()
	return act, nil
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

// PendingPlayers lists the display names of players who control a character but
// have not yet submitted an action this round.
func (s *SessionState) PendingPlayers() []string {
	s.mu.Lock()
	defer s.mu.Unlock()
	acted := map[string]bool{}
	if s.Round != nil {
		for _, a := range s.Round.Actions {
			acted[a.PlayerID] = true
		}
	}
	var pending []string
	for pid, slot := range s.Players {
		if !acted[pid] {
			pending = append(pending, slot.DisplayName)
		}
	}
	return pending
}

// Controllers returns a map of character name → controlling player's display
// name, for showing who plays whom.
func (s *SessionState) Controllers() map[string]string {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make(map[string]string, len(s.Players))
	for _, slot := range s.Players {
		if slot.CharacterName != "" {
			out[slot.CharacterName] = slot.DisplayName
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
