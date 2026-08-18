package domain

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"
	"unicode"

	"github.com/theburrowhub/thaimaturgy/internal/rules"
)

const (
	// maxWorldChangeLen bounds a single DM-recorded change so one call can't blow
	// up the prompt; maxWorldChangesPerTarget caps how many are retained (and thus
	// rendered) per entity, keeping the always-on grounding bounded (issue #21,
	// Heimdallm review).
	maxWorldChangeLen        = 400
	maxWorldChangesPerTarget = 12
	// maxWorldDescriptionLen bounds a full current-description override (v2). It is
	// generous (a rewritten room/NPC description) but still capped so one call
	// can't blow up the prompt or store unbounded untrusted text.
	maxWorldDescriptionLen = 4000
)

// sanitizeWorldChange normalizes a DM-recorded change for safe, bounded storage.
// The text is model-generated in response to player actions, so it is treated as
// UNTRUSTED: all whitespace (including newlines) is collapsed to single spaces and
// control characters are stripped, so a persisted change is a single line that
// cannot inject extra lines, headings, or role/fence markers into the prompt; the
// result is then length-capped. Returns "" when nothing printable remains.
func sanitizeWorldChange(s string) string {
	var b strings.Builder
	lastSpace := false
	for _, r := range s {
		if unicode.IsSpace(r) {
			if !lastSpace {
				b.WriteRune(' ')
				lastSpace = true
			}
			continue
		}
		if unicode.IsControl(r) {
			continue
		}
		b.WriteRune(r)
		lastSpace = false
	}
	out := strings.TrimSpace(b.String())
	// Cap by RUNES (not bytes) so truncation never splits a multi-byte character
	// and stores invalid UTF-8 (which JSON would then corrupt).
	if r := []rune(out); len(r) > maxWorldChangeLen {
		out = strings.TrimSpace(string(r[:maxWorldChangeLen])) + "…"
	}
	return out
}

// sanitizeWorldDescription normalizes a full current-description override (v2).
// Like sanitizeWorldChange the text is UNTRUSTED (model-generated), but a full
// description may legitimately span paragraphs, so newlines are preserved (other
// control chars and tabs are dropped/spaced); runs of 3+ blank lines are
// collapsed, and the result is length-capped by runes.
func sanitizeWorldDescription(s string) string {
	var b strings.Builder
	for _, r := range s {
		switch {
		case r == '\n':
			b.WriteRune('\n')
		case r == '\t':
			b.WriteRune(' ')
		case unicode.IsControl(r):
			// drop other control characters (can't inject role/fence markers)
		default:
			b.WriteRune(r)
		}
	}
	out := strings.TrimSpace(b.String())
	for strings.Contains(out, "\n\n\n") {
		out = strings.ReplaceAll(out, "\n\n\n", "\n\n")
	}
	// Neutralize runs of 3+ hyphens: with newlines preserved, an attacker-crafted
	// description could otherwise forge the "--- END CURRENT WORLD STATE ---"
	// delimiter on its own line and break out of the untrusted data block. Killing
	// every 3+ hyphen run makes a fence line impossible to reconstruct.
	for strings.Contains(out, "---") {
		out = strings.ReplaceAll(out, "---", "—")
	}
	if r := []rune(out); len(r) > maxWorldDescriptionLen {
		out = strings.TrimSpace(string(r[:maxWorldDescriptionLen])) + "…"
	}
	return out
}

// LogEntryType classifies an entry in the session timeline.
type LogEntryType string

const (
	LogNote     LogEntryType = "note"     // free-form DM note
	LogLocation LogEntryType = "location" // party moved
	LogNPC      LogEntryType = "npc"      // NPC met / disposition / death
	LogEvent    LogEntryType = "event"    // scripted event triggered
	LogFlag     LogEntryType = "flag"     // flag/variable set
	LogRoll     LogEntryType = "roll"     // dice roll / check
	LogQuest    LogEntryType = "quest"    // quest progress
	LogParty    LogEntryType = "party"    // party member update
	LogSystem   LogEntryType = "system"   // system message
	LogChat     LogEntryType = "chat"     // in-character player dialogue (context, not an action)
	LogWorld    LogEntryType = "world"    // DM-recorded consequence changing the authored world
)

// LogEntry is a single event in the running session timeline — either a
// free-form note the DM wrote or a structured marker of something that
// happened at the table.
type LogEntry struct {
	Type      LogEntryType   `json:"type"`
	Message   string         `json:"message"`
	Data      map[string]any `json:"data,omitempty"`
	Timestamp time.Time      `json:"timestamp"`
}

// SessionLog is a bounded timeline of LogEntry.
type SessionLog struct {
	Entries []LogEntry `json:"entries"`
	MaxSize int        `json:"max_size"`
}

// NewSessionLog creates a log bounded to maxSize entries.
func NewSessionLog(maxSize int) *SessionLog {
	if maxSize <= 0 {
		maxSize = 500
	}
	return &SessionLog{Entries: []LogEntry{}, MaxSize: maxSize}
}

// Add appends an entry, trimming oldest entries past MaxSize.
func (l *SessionLog) Add(entry LogEntry) {
	if entry.Timestamp.IsZero() {
		entry.Timestamp = time.Now()
	}
	l.Entries = append(l.Entries, entry)
	// MaxSize <= 0 means "keep everything" so the full timeline persists across
	// sessions; consumers that feed the model use GetLast to bound what they send.
	if l.MaxSize > 0 && len(l.Entries) > l.MaxSize {
		l.Entries = l.Entries[len(l.Entries)-l.MaxSize:]
	}
}

// GetLast returns the last n entries (all if n<=0 or n>len).
func (l *SessionLog) GetLast(n int) []LogEntry {
	if n <= 0 || n > len(l.Entries) {
		return l.Entries
	}
	return l.Entries[len(l.Entries)-n:]
}

// Len returns the number of entries.
func (l *SessionLog) Len() int { return len(l.Entries) }

// SessionMode selects how the oracle behaves during a session.
type SessionMode string

const (
	// ModeAssistant is the default: the AI is an oracle assisting a human DM who
	// runs the game for real players.
	ModeAssistant SessionMode = "assistant"
	// ModeVirtualDM turns the AI into the Dungeon Master running the game for a
	// solo player. Its primary use is to playtest / debug an adventure by playing
	// through it. The mode is toggleable in-session.
	ModeVirtualDM SessionMode = "dm"
)

// NPCStatus tracks the running state of an NPC the party has interacted with.
type NPCStatus struct {
	Met         bool   `json:"met"`
	Disposition string `json:"disposition,omitempty"`
	Alive       bool   `json:"alive"`
	Notes       string `json:"notes,omitempty"`
}

// PartyMember is the DM's lightweight tracking of a player character.
type PartyMember struct {
	Name      string `json:"name"`
	Class     string `json:"class,omitempty"`
	Level     int    `json:"level,omitempty"`
	CurrentHP int    `json:"current_hp,omitempty"`
	MaxHP     int    `json:"max_hp,omitempty"`
	AC        int    `json:"ac,omitempty"`
	Notes     string `json:"notes,omitempty"`
}

// WorldChange is a single DM-recorded consequence layered on top of an authored
// entity (a room, zone, NPC, item…). The authored module stays immutable; the
// session accumulates these so later narration reflects what the party did (e.g.
// "the armor has been moved out of this room") instead of repeating a description
// the party already invalidated.
type WorldChange struct {
	Change    string    `json:"change"`
	Timestamp time.Time `json:"timestamp"`
}

// QuestProgress tracks a quest/objective's state at the table.
type QuestProgress struct {
	ID     string `json:"id"`
	Name   string `json:"name"`
	Status string `json:"status"` // active | completed | failed
	Notes  string `json:"notes,omitempty"`
}

// ErrRulesLockConflict means a session is already pinned to a different exact
// rules artifact. Changing that lock requires an explicit migration rather than
// rebinding the session in place.
var ErrRulesLockConflict = errors.New("domain: rules lock conflict")

// ErrRulesImportConflict reports an equal-generation persisted fork. The
// receiver remains authoritative and is never overwritten in this case.
var ErrRulesImportConflict = errors.New("domain: rules import conflict")

// RulesSession is the persisted transactional rules host for a running session.
// Lock pins one exact artifact; Revision advances only for reduced event batches;
// Generation also tracks metadata-only commits. InitialState plus EventBatches
// can replay State, while receipts, pending steps, and random draws make retries
// and external responses durable and auditable.
type RulesSession struct {
	Lock         rules.Lock               `json:"lock"`
	Revision     uint64                   `json:"revision"`
	Generation   uint64                   `json:"generation,omitempty"`
	Lineage      []string                 `json:"lineage,omitempty"`
	InitialState rules.Payload            `json:"initial_state"`
	State        rules.Payload            `json:"state"`
	Receipts     []RulesReceipt           `json:"receipts,omitempty"`
	Pending      []RulesPendingResolution `json:"pending,omitempty"`
	EventBatches []RulesEventBatch        `json:"event_batches,omitempty"`
	RandomDraws  []RulesRandomDraw        `json:"random_draws,omitempty"`
}

// Validate checks that the persisted rules state is complete and structurally
// valid without assigning any system-specific meaning to State.
func (r RulesSession) Validate() error {
	if err := r.Lock.Validate(); err != nil {
		return fmt.Errorf("rules session lock: %w", err)
	}
	if err := r.State.Validate(); err != nil {
		return fmt.Errorf("rules session state: %w", err)
	}
	if err := r.InitialState.Validate(); err != nil {
		return fmt.Errorf("rules session initial state: %w", err)
	}
	return r.validateRuntime()
}

// SessionState is the persisted, mutable record of a running play session of an
// adventure. It references the adventure module by ID (the immutable content
// lives in the Adventure struct, reloaded from disk).
type SessionState struct {
	Name           string `json:"name"`
	AdventureID    string `json:"adventure_id"`
	AdventureTitle string `json:"adventure_title"`

	// Rules is absent on legacy sessions. Once present, its exact lock may only be
	// changed by an explicit migration; BindRules never upgrades it silently.
	Rules *RulesSession `json:"rules,omitempty"`

	// Structured progress fed by the DM.
	CurrentZone     string                `json:"current_zone,omitempty"`
	CurrentRoom     string                `json:"current_room,omitempty"`
	CurrentScene    string                `json:"current_scene,omitempty"` // active scene/phase (empty when the module has no scenes)
	VisitedRooms    map[string]bool       `json:"visited_rooms,omitempty"`
	KnownNPCs       map[string]*NPCStatus `json:"known_npcs,omitempty"`
	TriggeredEvents map[string]bool       `json:"triggered_events,omitempty"`
	Flags           map[string]bool       `json:"flags,omitempty"`
	Variables       map[string]string     `json:"variables,omitempty"`
	Party           []*PartyMember        `json:"party,omitempty"`
	Quests          []QuestProgress       `json:"quests,omitempty"`

	// WorldEdits overlays DM-recorded consequences onto the immutable authored
	// module, keyed by an opaque target string the engine composes as
	// "<kind>:<id>" (kind ∈ room|zone|npc|item|event). Reads and narrations layer
	// these on top of the authored text so the world reflects the party's actions.
	// Empty for sessions that never edited the world (backward compatible).
	WorldEdits map[string][]WorldChange `json:"world_edits,omitempty"`

	// WorldDescriptions is the mutable-world v2: the DM-recorded CURRENT
	// player-facing description of a target ("kind:id", kind ∈ room|npc) that
	// wholly SUPERSEDES the authored text. When one is set, grounding shows ONLY
	// this (the authored read-aloud/appearance is suppressed), so the model never
	// sees a stale original alongside a "superseding" note — removing the confusion
	// and the size cap of the WorldEdits bullet log. Model-generated ⇒ delivered as
	// untrusted data, never in the system prompt. Empty for sessions that never set
	// a full description (backward compatible; WorldEdits still handles small
	// incremental consequences).
	WorldDescriptions map[string]string `json:"world_descriptions,omitempty"`

	// Mode selects oracle behaviour: assistant (default) or virtual DM. It can be
	// toggled at any point during a session.
	Mode SessionMode `json:"mode,omitempty"`

	// Characters is the player party used in virtual-DM mode (nil/empty in
	// assistant mode). PC is the legacy single-character field kept for loading old
	// sessions; it is migrated into Characters on first use (see EnsureParty).
	Characters []*Character `json:"characters,omitempty"`
	PC         *Character   `json:"pc,omitempty"`

	// Multiplayer (virtual-DM mode): Players maps a player id to the party member
	// they control; Round buffers each player's declared action for the current
	// turn until the DM resolves it. Both are empty in single-player use.
	Players map[string]*PlayerSlot `json:"players,omitempty"`
	Round   *TurnRound             `json:"round,omitempty"`
	// Started marks that the game has begun (the DM gave the opening scene). Before
	// it is set, a multiplayer front-end accepts only setup/start commands.
	Started bool `json:"started,omitempty"`
	// PendingAssignments reserves a character for a Telegram @username (lower-cased)
	// that hasn't picked yet; it binds to the real player when they next message.
	PendingAssignments map[string]string `json:"pending_assignments,omitempty"`

	// Free-form timeline and running summary.
	Log     *SessionLog `json:"log"`
	Summary string      `json:"summary,omitempty"`

	// The DM's dialogue with the oracle.
	Conversation *Conversation `json:"conversation"`

	CreatedAt time.Time     `json:"created_at"`
	UpdatedAt time.Time     `json:"updated_at"`
	PlayTime  time.Duration `json:"play_time"`

	// onLog, when set, is invoked for every timeline entry as it is added, so a
	// frontend can persist an append-only journal of the game as it happens. It is
	// unexported and therefore never serialized.
	onLog func(LogEntry)

	// mu guards concurrent mutation and serialization of the state. The oracle
	// runs in its own goroutine mutating the state through the tool router while a
	// frontend may read/serialize it (autosave); every exported mutator and the
	// JSON marshaller take this lock. Unexported (never serialized) and, being a
	// mutex, must not be copied — callers pass *SessionState.
	mu sync.Mutex

	// rulesInFlight coordinates the same idempotency key across every router
	// attached to this in-memory session. It is deliberately runtime-only: a
	// process crash releases unfinished work, while committed receipts below are
	// persisted and survive restarts.
	rulesInFlight map[string]*rulesRequestFlight
	rulesClaimSeq uint64
	// rulesHostMu serializes effectful rules drivers (including calls into RNG)
	// while the finer-grained generation CAS remains the correctness boundary
	// for direct/domain callers and imported subprocess state.
	rulesHostMu sync.Mutex
	// toolMutationMu keeps a legacy/world mutation and its durable MCP receipt in
	// one serialization window. MarshalJSON takes the same lock, so an autosave
	// cannot publish the effect without the receipt that makes retries idempotent.
	toolMutationMu sync.Mutex
}

// MarshalJSON serializes the state under the mutex so an autosave can't race a
// concurrent mutation from the oracle goroutine (which would otherwise risk a
// "concurrent map iteration and map write" panic).
func (s *SessionState) MarshalJSON() ([]byte, error) {
	s.toolMutationMu.Lock()
	defer s.toolMutationMu.Unlock()
	s.mu.Lock()
	defer s.mu.Unlock()
	type alias SessionState
	return json.Marshal((*alias)(s))
}

// UnmarshalJSON decodes the state and re-initializes the maps/pointers that use
// `omitempty` (they come back nil when the saved JSON omitted an empty value).
// Without this, a state reloaded from disk — notably in the Claude-CLI MCP tools
// subprocess — panics with "assignment to entry in nil map" the first time a tool
// records an NPC, flag, visited room, etc.
func (s *SessionState) UnmarshalJSON(b []byte) error {
	type alias SessionState
	if err := json.Unmarshal(b, (*alias)(s)); err != nil {
		return err
	}
	s.ensureInitialized()
	if s.Rules != nil {
		// Rules blocks written before the transactional host have no audit log,
		// so their current materialized state is also the only valid replay root.
		if s.Rules.InitialState.IsZero() && len(s.Rules.EventBatches) == 0 {
			s.Rules.InitialState = s.Rules.State
			// Earlier kernels exposed Revision but had no event journal capable of
			// explaining a non-zero value. Adopt the materialized state as the new
			// revision-zero root instead of manufacturing an unreplayable gap.
			s.Rules.Revision = 0
		}
		if err := s.Rules.Validate(); err != nil {
			return err
		}
	}
	return nil
}

// ensureInitialized guarantees the structured maps and the log/conversation
// pointers are non-nil, so every mutator is safe on a freshly loaded state.
func (s *SessionState) ensureInitialized() {
	if s.VisitedRooms == nil {
		s.VisitedRooms = make(map[string]bool)
	}
	if s.KnownNPCs == nil {
		s.KnownNPCs = make(map[string]*NPCStatus)
	}
	if s.TriggeredEvents == nil {
		s.TriggeredEvents = make(map[string]bool)
	}
	if s.Flags == nil {
		s.Flags = make(map[string]bool)
	}
	if s.Variables == nil {
		s.Variables = make(map[string]string)
	}
	if s.WorldEdits == nil {
		s.WorldEdits = make(map[string][]WorldChange)
	}
	if s.WorldDescriptions == nil {
		s.WorldDescriptions = make(map[string]string)
	}
	if s.Log == nil {
		s.Log = &SessionLog{Entries: []LogEntry{}, MaxSize: 0}
	}
	if s.Conversation == nil {
		s.Conversation = &Conversation{Messages: []Message{}, MaxSize: 0}
	}
	if s.rulesInFlight == nil {
		s.rulesInFlight = make(map[string]*rulesRequestFlight)
	}
	// Upgrade legacy single-character player slots to the multi-character model (#29).
	s.migratePlayerSlots()
}

// SetLogHook registers a callback invoked for every timeline entry the moment it
// is added — used to write a persistent, as-it-happens session journal.
func (s *SessionState) SetLogHook(fn func(LogEntry)) { s.onLog = fn }

// record adds an entry to the (bounded) in-memory timeline and notifies the log
// hook (which may persist it to an unbounded journal).
func (s *SessionState) record(e LogEntry) {
	if e.Timestamp.IsZero() {
		e.Timestamp = time.Now()
	}
	s.Log.Add(e)
	if s.onLog != nil {
		s.onLog(e)
	}
}

// NewSessionState creates a fresh session for an adventure, defaulting the
// current location to the first room of the first zone when available.
func NewSessionState(name string, adv *Adventure) *SessionState {
	now := time.Now()
	s := &SessionState{
		Name:              name,
		VisitedRooms:      make(map[string]bool),
		KnownNPCs:         make(map[string]*NPCStatus),
		TriggeredEvents:   make(map[string]bool),
		Flags:             make(map[string]bool),
		Variables:         make(map[string]string),
		WorldEdits:        make(map[string][]WorldChange),
		WorldDescriptions: make(map[string]string),
		// Unbounded (MaxSize 0): persist the complete timeline and conversation so a
		// session can be reopened with full context. The oracle only sends a recent
		// window to the model, so unbounded storage doesn't grow the prompt.
		Log:          &SessionLog{Entries: []LogEntry{}, MaxSize: 0},
		Conversation: &Conversation{Messages: []Message{}, MaxSize: 0},
		CreatedAt:    now,
		UpdatedAt:    now,
	}
	if adv != nil {
		s.AdventureID = adv.ID
		s.AdventureTitle = adv.Title
		// Start at the authored entry room (falls back to the first room) rather
		// than blindly at Zones[0].Rooms[0], so the starting position doesn't
		// depend on the order zones/rooms happen to be written.
		if rid := adv.StartRoomID(); rid != "" {
			if room, zone := adv.Room(rid); room != nil && zone != nil {
				s.CurrentZone = zone.ID
				s.CurrentRoom = room.ID
				s.VisitedRooms[room.ID] = true
			}
		}
		// Start in the module's initial scene (empty when it authored no scenes).
		s.CurrentScene = adv.InitialSceneID()
	}
	return s
}

func (s *SessionState) touch() { s.UpdatedAt = time.Now() }

// SetLocation records the party's current zone and room, marking the room
// visited and logging the move.
// Location returns the party's current zone and room ids under the lock, for
// safe reads from goroutines that run concurrently with SetLocation (e.g. the
// Telegram command handler while an oracle turn resolves).
func (s *SessionState) Location() (zone, room string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.CurrentZone, s.CurrentRoom
}

func (s *SessionState) SetLocation(zoneID, roomID, roomName string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.CurrentZone = zoneID
	s.CurrentRoom = roomID
	if roomID != "" {
		s.VisitedRooms[roomID] = true
	}
	label := roomName
	if label == "" {
		label = roomID
	}
	s.record(LogEntry{Type: LogLocation, Message: "Entered " + label,
		Data: map[string]any{"zone": zoneID, "room": roomID}})
	s.touch()
}

// Scene returns the active scene id.
func (s *SessionState) Scene() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.CurrentScene
}

// SetScene switches the active narrative scene/phase. The change is recorded as a
// LogSystem entry (which /recap and player-facing views filter out, so a scene's
// authored name can't leak as a spoiler).
func (s *SessionState) SetScene(id, name string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.CurrentScene = id
	label := name
	if label == "" {
		label = id
	}
	s.record(LogEntry{Type: LogSystem, Message: "Scene: " + label,
		Data: map[string]any{"scene": id}})
	s.touch()
}

// MeetNPC marks an NPC as met (creating status if needed) and returns it.
func (s *SessionState) MeetNPC(id, name string) *NPCStatus {
	s.mu.Lock()
	defer s.mu.Unlock()
	st := s.KnownNPCs[id]
	if st == nil {
		st = &NPCStatus{Alive: true}
		s.KnownNPCs[id] = st
	}
	if !st.Met {
		st.Met = true
		label := name
		if label == "" {
			label = id
		}
		s.record(LogEntry{Type: LogNPC, Message: "Met " + label,
			Data: map[string]any{"npc": id}})
	}
	s.touch()
	return st
}

// NPCKnown reports whether the party has met the NPC, under the lock (safe to
// call from goroutines running concurrently with the oracle turn).
func (s *SessionState) NPCKnown(id string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	st := s.KnownNPCs[id]
	return st != nil && st.Met
}

// npcState is the lock-free core of NPCState, for callers already holding s.mu.
func (s *SessionState) npcState(id string) *NPCStatus {
	st := s.KnownNPCs[id]
	if st == nil {
		st = &NPCStatus{Alive: true}
		s.KnownNPCs[id] = st
	}
	return st
}

// NPCState returns the tracked status for an NPC, creating a default if absent.
func (s *SessionState) NPCState(id string) *NPCStatus {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.npcState(id)
}

// SetNPCDisposition records an NPC's disposition under the lock (the caller must
// not mutate the returned NPCStatus directly, which would bypass synchronization).
func (s *SessionState) SetNPCDisposition(id, disposition string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.npcState(id).Disposition = disposition
	s.record(LogEntry{Type: LogNPC, Message: id + " disposition → " + disposition,
		Data: map[string]any{"npc": id}})
	s.touch()
}

// SetNPCAlive records whether an NPC is alive under the lock.
func (s *SessionState) SetNPCAlive(id string, alive bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.npcState(id).Alive = alive
	status := "alive"
	if !alive {
		status = "dead"
	}
	s.record(LogEntry{Type: LogNPC, Message: id + " is now " + status,
		Data: map[string]any{"npc": id}})
	s.touch()
}

// migratePC moves a legacy single PC into the party. Caller holds s.mu.
func (s *SessionState) migratePC() {
	if s.PC != nil {
		s.Characters = append(s.Characters, s.PC)
		s.PC = nil
	}
}

// resolveCharacter finds a party member by name (case-insensitive). An empty name
// returns the sole member when the party has exactly one. Caller holds s.mu.
func (s *SessionState) resolveCharacter(name string) *Character {
	s.migratePC()
	name = strings.TrimSpace(name)
	if name == "" {
		if len(s.Characters) == 1 {
			return s.Characters[0]
		}
		return nil
	}
	for _, c := range s.Characters {
		if c != nil && strings.EqualFold(c.Name, name) {
			return c
		}
	}
	return nil
}

// MutateCharacter runs fn against the named party member under the lock and
// returns the resolved name and whether a member matched. An empty name targets
// the sole member when the party has exactly one (ambiguous otherwise).
func (s *SessionState) MutateCharacter(name string, fn func(*Character)) (string, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	c := s.resolveCharacter(name)
	if c == nil {
		return "", false
	}
	fn(c)
	s.touch()
	return c.Name, true
}

// AddUserMessage appends a user message to the conversation under the lock.
func (s *SessionState) AddUserMessage(content string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.Conversation.AddUserMessage(content)
}

// AddAssistantMessage appends an assistant message to the conversation under the
// lock.
func (s *SessionState) AddAssistantMessage(content string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.Conversation.AddAssistantMessage(content)
}

// RecentLog returns a copy of the last n timeline entries under the lock, so a
// reader (e.g. a UI panel) never iterates the slice while a writer appends.
func (s *SessionState) RecentLog(n int) []LogEntry {
	s.mu.Lock()
	defer s.mu.Unlock()
	last := s.Log.GetLast(n)
	out := make([]LogEntry, len(last))
	copy(out, last)
	return out
}

// LogLen returns the timeline length under the lock.
func (s *SessionState) LogLen() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.Log.Len()
}

// ImportStructured replaces the structured progress fields from src under the
// lock. Used by the Claude-CLI merge path, where a subprocess mutated a copy of
// the state; the timeline is replayed separately via AppendLog. Equal-generation
// divergent rules blocks are preserved locally and reported as a fork.
func (s *SessionState) ImportStructured(src *SessionState) {
	_ = s.ImportStructuredChecked(src)
}

// ImportRulesRuntimeChecked reconciles only the transactional rules block from
// src. It is used by process handoffs that must preserve the receiver's ordinary
// session fields while adopting a newer durable rules checkpoint. Older source
// generations are ignored, equal-generation forks and lock changes fail closed,
// and an absent legacy source never removes an existing binding.
func (s *SessionState) ImportRulesRuntimeChecked(src *SessionState) (bool, error) {
	if src == nil {
		return false, errors.New("domain: nil rules import source")
	}
	var importedRules *RulesSession
	runtime, exists, err := src.RulesRuntimeSnapshotStrict()
	if err != nil {
		return false, fmt.Errorf("import rules runtime: %w", err)
	}
	if exists {
		importedRules = &runtime
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	return s.importRulesRuntimeLocked(importedRules)
}

// ImportStructuredChecked is ImportStructured with explicit reporting for a
// lock mismatch or equal-generation rules fork.
func (s *SessionState) ImportStructuredChecked(src *SessionState) error {
	if src == nil {
		return errors.New("domain: nil structured import source")
	}
	var importedRules *RulesSession
	runtime, exists, err := src.RulesRuntimeSnapshotStrict()
	if err != nil {
		return fmt.Errorf("import rules runtime: %w", err)
	}
	if exists {
		importedRules = &runtime
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	// Reject rules conflicts before copying any other structured field. A failed
	// merge is all-or-nothing from the caller's perspective.
	if _, err := s.importRulesRuntimeLocked(importedRules); err != nil {
		return err
	}
	s.CurrentZone = src.CurrentZone
	s.CurrentRoom = src.CurrentRoom
	// Don't let an empty imported scene clobber an already-seeded active scene
	// (e.g. a merge from a subprocess that didn't set it).
	if src.CurrentScene != "" {
		s.CurrentScene = src.CurrentScene
	}
	s.VisitedRooms = src.VisitedRooms
	s.KnownNPCs = src.KnownNPCs
	s.TriggeredEvents = src.TriggeredEvents
	s.Flags = src.Flags
	s.Variables = src.Variables
	s.WorldEdits = src.WorldEdits
	s.WorldDescriptions = src.WorldDescriptions
	s.Party = src.Party
	s.Quests = src.Quests
	s.Characters = src.Characters
	s.PC = src.PC
	return nil
}

// importRulesRuntimeLocked implements the monotonic rules merge shared by full
// structured imports and rules-only process-handoff reconciliation. s.mu must be
// held by the caller.
func (s *SessionState) importRulesRuntimeLocked(importedRules *RulesSession) (bool, error) {
	if importedRules == nil {
		return false, nil
	}
	if s.Rules != nil {
		if err := s.Rules.Validate(); err != nil {
			return false, fmt.Errorf("current rules runtime: %w", err)
		}
		switch {
		case s.Rules.Lock != importedRules.Lock:
			return false, ErrRulesLockConflict
		case importedRules.Generation <= s.Rules.Generation:
			if importedRules.Generation == s.Rules.Generation {
				if err := importedRules.ValidateDescendantOf(*s.Rules); err != nil {
					return false, err
				}
			}
			return false, nil
		}
		if err := importedRules.ValidateDescendantOf(*s.Rules); err != nil {
			return false, err
		}
	}
	cp := cloneRulesSession(*importedRules)
	s.Rules = &cp
	return true, nil
}

// BindRules pins the session to lock and seeds its opaque state. It returns true
// only when a new binding was created. Repeating the exact lock is an idempotent
// no-op that preserves the current state and revision; another lock is rejected.
func (s *SessionState) BindRules(lock rules.Lock, state rules.Payload) (bool, error) {
	candidate := RulesSession{Lock: lock, InitialState: state, State: state}
	if err := candidate.Validate(); err != nil {
		return false, err
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	if s.Rules == nil {
		s.Rules = &candidate
		s.touch()
		return true, nil
	}
	if err := s.Rules.Validate(); err != nil {
		return false, err
	}
	if s.Rules.Lock != lock {
		return false, fmt.Errorf("%w: session has %s@%s (%s), requested %s@%s (%s)",
			ErrRulesLockConflict,
			s.Rules.Lock.ID, s.Rules.Lock.Version, s.Rules.Lock.Digest,
			lock.ID, lock.Version, lock.Digest)
	}
	return false, nil
}

// RulesSnapshot returns an immutable value snapshot of the currently pinned
// rules state. The boolean is false for legacy sessions and invalid in-memory
// values; loaded and BindRules-created sessions maintain this invariant.
func (s *SessionState) RulesSnapshot() (rules.Snapshot, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.Rules == nil {
		return rules.Snapshot{}, false
	}
	snapshot := rules.Snapshot{
		Ruleset:  s.Rules.Lock,
		Revision: s.Rules.Revision,
		State:    s.Rules.State,
	}
	if err := snapshot.Validate(); err != nil {
		return rules.Snapshot{}, false
	}
	return snapshot, true
}

// LockRulesHost serializes one complete effectful gateway resolution. The
// returned release function must be called exactly once. It intentionally does
// not hold the state mutex while a ruleset, RNG provider, or persistence hook
// executes.
func (s *SessionState) LockRulesHost() func() {
	s.rulesHostMu.Lock()
	return s.rulesHostMu.Unlock
}

// LockToolMutation serializes one non-rules ToolRouter mutation with its
// receipt and with JSON serialization. Rules drivers use LockRulesHost as the
// outer lock, preserving a single lock order when the two paths share receipts.
func (s *SessionState) LockToolMutation() func() {
	s.toolMutationMu.Lock()
	return s.toolMutationMu.Unlock
}

// TriggerEvent records that a scripted event has fired.
func (s *SessionState) TriggerEvent(id, name string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.TriggeredEvents[id] {
		return
	}
	s.TriggeredEvents[id] = true
	label := name
	if label == "" {
		label = id
	}
	s.record(LogEntry{Type: LogEvent, Message: "Triggered event: " + label,
		Data: map[string]any{"event": id}})
	s.touch()
}

// SetFlag sets a boolean flag and logs it.
func (s *SessionState) SetFlag(key string, value bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.Flags[key] = value
	s.record(LogEntry{Type: LogFlag, Message: "Flag " + key + " set",
		Data: map[string]any{"key": key, "value": value}})
	s.touch()
}

// SetVariable sets a string variable and logs it.
func (s *SessionState) SetVariable(key, value string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.Variables[key] = value
	s.record(LogEntry{Type: LogFlag, Message: "Variable " + key + " = " + value,
		Data: map[string]any{"key": key, "value": value}})
	s.touch()
}

// RecordWorldChange appends a DM-recorded consequence to an authored entity,
// keyed by the opaque target (composed by the engine as "<kind>:<id>"). label is
// a human-readable name for the timeline entry (falls back to the target). The
// change text is sanitized (single line, control chars stripped, length-capped)
// and the per-target history is bounded, so a persisted change can neither
// inject prompt structure nor grow the always-on grounding without limit. It
// records a LogWorld entry for traceability and reports whether anything was
// stored (false when the change is empty after sanitizing). The authored module
// is never mutated — the change lives only in the session overlay.
func (s *SessionState) RecordWorldChange(target, label, change string) bool {
	change = sanitizeWorldChange(change)
	if change == "" {
		return false
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.WorldEdits == nil {
		s.WorldEdits = make(map[string][]WorldChange)
	}
	list := append(s.WorldEdits[target], WorldChange{Change: change, Timestamp: time.Now()})
	// Keep only the most recent entries: the latest changes reflect the current
	// state, and this caps how much is rendered into the prompt each turn.
	if len(list) > maxWorldChangesPerTarget {
		list = list[len(list)-maxWorldChangesPerTarget:]
	}
	s.WorldEdits[target] = list
	name := label
	if name == "" {
		name = target
	}
	s.record(LogEntry{Type: LogWorld, Message: "World change (" + name + "): " + change,
		Data: map[string]any{"target": target}})
	s.touch()
	return true
}

// WorldChangesFor returns a copy of the changes recorded for a target under the
// lock (nil when none), so callers never iterate the slice while a writer appends.
func (s *SessionState) WorldChangesFor(target string) []WorldChange {
	s.mu.Lock()
	defer s.mu.Unlock()
	src := s.WorldEdits[target]
	if len(src) == 0 {
		return nil
	}
	out := make([]WorldChange, len(src))
	copy(out, src)
	return out
}

// SetWorldDescription records the CURRENT full player-facing description of a
// target ("kind:id"), superseding the authored text (mutable-world v2). A blank
// description CLEARS the override (revert to authored). Returns the stored
// (sanitized) description and whether an override remains set.
func (s *SessionState) SetWorldDescription(target, desc string) (string, bool) {
	clean := sanitizeWorldDescription(desc)
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.WorldDescriptions == nil {
		s.WorldDescriptions = make(map[string]string)
	}
	if clean == "" {
		delete(s.WorldDescriptions, target)
		s.record(LogEntry{Type: LogWorld, Message: "World description cleared (" + target + ")",
			Data: map[string]any{"target": target}})
		s.touch()
		return "", false
	}
	s.WorldDescriptions[target] = clean
	// The full text isn't logged (avoid noise/spoilers in the timeline); LogWorld
	// entries are filtered from the recent-timeline dump and from /recap.
	s.record(LogEntry{Type: LogWorld, Message: "World description updated (" + target + ")",
		Data: map[string]any{"target": target}})
	s.touch()
	return clean, true
}

// WorldDescription returns the current-description override for a target under
// the lock ("" when none is set).
func (s *SessionState) WorldDescription(target string) string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.WorldDescriptions[target]
}

// AppendLog appends a pre-formed timeline entry — used to replay into the live
// state mutations performed by an external tool process (e.g. the MCP tools
// server) so the log hook (journal) fires for them too.
func (s *SessionState) AppendLog(e LogEntry) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.record(e)
	s.touch()
}

// AddNote appends a free-form DM note to the timeline.
func (s *SessionState) AddNote(text string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.record(LogEntry{Type: LogNote, Message: text})
	s.touch()
}

// AddChat records an in-character line (roleplay) into the timeline as context
// for the DM, WITHOUT creating a round action or requiring a /dm resolution.
// speaker is the character's name (empty for an unattributed line).
func (s *SessionState) AddChat(speaker, text string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	msg := strings.TrimSpace(text)
	if sp := strings.TrimSpace(speaker); sp != "" {
		msg = sp + ": " + msg
	}
	s.record(LogEntry{Type: LogChat, Message: msg})
	s.touch()
}

// RestParty applies a rest to the whole party (or a single named character) and
// records it in the timeline so it reaches the DM. long=true is a long rest;
// otherwise a short rest where each character spends up to shortDice hit dice
// (shortDice<=0 spends all remaining). Returns a human-readable summary.
func (s *SessionState) RestParty(long bool, characterName string, shortDice int) string {
	s.mu.Lock()
	defer s.mu.Unlock()
	target := strings.TrimSpace(characterName)
	var lines []string
	for _, c := range s.Characters {
		if c == nil {
			continue
		}
		if target != "" && !strings.EqualFold(c.Name, target) {
			continue
		}
		if long {
			c.LongRest()
			lines = append(lines, fmt.Sprintf("%s: full HP (%d/%d)", c.Name, c.CurrentHP, c.MaxHP))
		} else {
			healed, spent := c.ShortRest(shortDice)
			lines = append(lines, fmt.Sprintf("%s: +%d HP (%d/%d), %d hit dice spent", c.Name, healed, c.CurrentHP, c.MaxHP, spent))
		}
	}
	kind := "short rest"
	if long {
		kind = "long rest"
	}
	summary := "No party members to rest."
	if target != "" && len(lines) == 0 {
		summary = "No party member named " + target + "."
	}
	if len(lines) > 0 {
		summary = "The party takes a " + kind + ".\n" + strings.Join(lines, "\n")
	}
	s.record(LogEntry{Type: LogParty, Message: summary})
	s.touch()
	return summary
}

// AdvanceQuest creates or updates a quest's progress.
func (s *SessionState) AdvanceQuest(id, name, status string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for i := range s.Quests {
		if s.Quests[i].ID == id {
			s.Quests[i].Status = status
			if name != "" {
				s.Quests[i].Name = name
			}
			s.record(LogEntry{Type: LogQuest, Message: "Quest '" + s.Quests[i].Name + "' → " + status})
			s.touch()
			return
		}
	}
	s.Quests = append(s.Quests, QuestProgress{ID: id, Name: name, Status: status})
	s.record(LogEntry{Type: LogQuest, Message: "New quest: " + name})
	s.touch()
}

// effectiveMode is the lock-free core of EffectiveMode, for callers that already
// hold s.mu.
func (s *SessionState) effectiveMode() SessionMode {
	if s.Mode == "" {
		return ModeAssistant
	}
	return s.Mode
}

// EffectiveMode returns the session mode, treating the empty zero value as the
// default assistant mode.
func (s *SessionState) EffectiveMode() SessionMode {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.effectiveMode()
}

// SetMode switches the session to the given mode and logs the change.
func (s *SessionState) SetMode(m SessionMode) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.effectiveMode() == m {
		return
	}
	s.Mode = m
	label := "Oracle (assistant to human DM)"
	if m == ModeVirtualDM {
		label = "Virtual DM (AI runs the game)"
	}
	s.record(LogEntry{Type: LogSystem, Message: "Mode switched to " + label,
		Data: map[string]any{"mode": string(m)}})
	s.touch()
}

// ToggleMode flips between assistant and virtual-DM mode and returns the new
// mode. It delegates to the (locking) EffectiveMode/SetMode, so it takes no lock
// itself.
func (s *SessionState) ToggleMode() SessionMode {
	if s.EffectiveMode() == ModeVirtualDM {
		s.SetMode(ModeAssistant)
	} else {
		s.SetMode(ModeVirtualDM)
	}
	return s.EffectiveMode()
}

// EnsureParty guarantees the player party exists — migrating a legacy PC or
// generating the default heterogeneous level-1 party — and reports whether it
// created the party just now. This is the single entry point both frontends and
// the tool router use, so the default roster lives in one place.
func (s *SessionState) EnsureParty() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.migratePC()
	if len(s.Characters) > 0 {
		return false
	}
	s.Characters = DefaultParty()
	return true
}

// SetParty replaces the whole player party under the lock and logs it.
func (s *SessionState) SetParty(party []*Character) {
	s.mu.Lock()
	defer s.mu.Unlock()
	EnsureUniqueNames(party)
	s.PC = nil
	s.Characters = party
	names := make([]string, 0, len(party))
	for _, c := range party {
		if c != nil {
			names = append(names, c.Name)
		}
	}
	s.record(LogEntry{Type: LogParty, Message: "Party set: " + strings.Join(names, ", ")})
	s.touch()
}

// LinkRosterIDs assigns roster IDs to party members by position (ids[i] → the
// i-th party member), skipping empty IDs and out-of-range indices. Linking by
// index (not name) is unambiguous even if two members share a name.
func (s *SessionState) LinkRosterIDs(ids []string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	changed := false
	for i, c := range s.Characters {
		if i >= len(ids) || ids[i] == "" || c == nil {
			continue
		}
		if c.ID != ids[i] {
			c.ID = ids[i]
			changed = true
		}
	}
	if changed {
		s.touch()
	}
}

// PartyNames returns the party members' names (copy) under the lock.
func (s *SessionState) PartyNames() []string {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]string, 0, len(s.Characters))
	for _, c := range s.Characters {
		if c != nil {
			out = append(out, c.Name)
		}
	}
	return out
}

// PartySnapshot returns deep value copies of the party members under the lock, so
// a reader (UI panel, prompt builder) never races a concurrent mutation.
func (s *SessionState) PartySnapshot() []Character {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.migratePC()
	out := make([]Character, 0, len(s.Characters))
	for _, c := range s.Characters {
		if c == nil {
			continue
		}
		cp := *c
		cp.Skills = append([]Skill(nil), c.Skills...)
		cp.Inventory = append([]InventoryItem(nil), c.Inventory...)
		cp.Conditions = append([]Condition(nil), c.Conditions...)
		cp.SavingThrows = append([]Ability(nil), c.SavingThrows...)
		cp.Languages = append([]string(nil), c.Languages...)
		cp.Proficiencies = append([]string(nil), c.Proficiencies...)
		cp.Features = append([]Trait(nil), c.Features...)
		// Deep-copy the spellcasting block (including its spellbook slice) so the
		// snapshot never aliases the live pointer a writer might mutate.
		if c.Spellcasting != nil {
			scCopy := *c.Spellcasting
			scCopy.Spells = append([]Spell(nil), c.Spellcasting.Spells...)
			cp.Spellcasting = &scCopy
		}
		out = append(out, cp)
	}
	return out
}

// UpsertPartyMember creates or updates a tracked party member under the lock.
// Non-nil pointers overwrite the corresponding field; a non-empty notes replaces
// the note.
func (s *SessionState) UpsertPartyMember(name string, currentHP, maxHP, ac *int, notes string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	var pm *PartyMember
	for _, m := range s.Party {
		if strings.EqualFold(m.Name, name) {
			pm = m
			break
		}
	}
	if pm == nil {
		pm = &PartyMember{Name: name}
		s.Party = append(s.Party, pm)
	}
	if currentHP != nil {
		pm.CurrentHP = *currentHP
	}
	if maxHP != nil {
		pm.MaxHP = *maxHP
	}
	if ac != nil {
		pm.AC = *ac
	}
	if notes != "" {
		pm.Notes = notes
	}
	s.record(LogEntry{Type: LogParty, Message: "Updated " + name})
	s.touch()
}

// Session is the runtime wrapper binding a persisted SessionState to its loaded
// Adventure module and the active Config.
type Session struct {
	State      *SessionState
	Adventure  *Adventure
	Config     *Config
	StartedAt  time.Time
	IsModified bool

	// RulesResolver is the runtime-only catalog that can look up the exact rules
	// artifact pinned in State. Persisted sessions carry the immutable lock, not
	// executable package implementations.
	RulesResolver rules.Resolver
	// DataDirectory is the runtime-only root from which this session's rules
	// catalog and other process-owned assets were loaded.
	DataDirectory string

	// PersistRules is an optional runtime-only durability barrier. Transactional
	// rules gateways invoke it after every applied checkpoint/receipt and before
	// resuming a ruleset or returning success. Frontends that promise crash
	// durability wire this to their atomic session-store replacement operation.
	// A nil callback retains the historical in-memory/autosave behaviour.
	PersistRules func(*SessionState) error
	modifiedMu   sync.Mutex
}

// NewSession binds state, adventure, and config into a runtime session.
func NewSession(state *SessionState, adv *Adventure, config *Config) *Session {
	// Seed the active scene when it's missing — e.g. resuming a pre-scenes save
	// against an adventure that now defines scenes — so scene overrides apply from
	// the start instead of only after a manual /scene. No-op for fresh sessions
	// (already seeded) and scene-less adventures (InitialSceneID is empty). Safe
	// without the lock: the session isn't shared yet at construction.
	if state != nil && adv != nil && state.CurrentScene == "" {
		state.CurrentScene = adv.InitialSceneID()
	}
	return &Session{
		State:     state,
		Adventure: adv,
		Config:    config,
		StartedAt: time.Now(),
	}
}

// MarkModified flags the session dirty and touches the timestamp.
func (s *Session) MarkModified() {
	s.modifiedMu.Lock()
	defer s.modifiedMu.Unlock()
	s.IsModified = true
	s.State.mu.Lock()
	s.State.touch()
	s.State.mu.Unlock()
}
