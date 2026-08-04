package domain

import (
	"encoding/json"
	"strings"
	"sync"
	"time"
)

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

// QuestProgress tracks a quest/objective's state at the table.
type QuestProgress struct {
	ID     string `json:"id"`
	Name   string `json:"name"`
	Status string `json:"status"` // active | completed | failed
	Notes  string `json:"notes,omitempty"`
}

// SessionState is the persisted, mutable record of a running play session of an
// adventure. It references the adventure module by ID (the immutable content
// lives in the Adventure struct, reloaded from disk).
type SessionState struct {
	Name           string `json:"name"`
	AdventureID    string `json:"adventure_id"`
	AdventureTitle string `json:"adventure_title"`

	// Structured progress fed by the DM.
	CurrentZone     string                `json:"current_zone,omitempty"`
	CurrentRoom     string                `json:"current_room,omitempty"`
	VisitedRooms    map[string]bool       `json:"visited_rooms,omitempty"`
	KnownNPCs       map[string]*NPCStatus `json:"known_npcs,omitempty"`
	TriggeredEvents map[string]bool       `json:"triggered_events,omitempty"`
	Flags           map[string]bool       `json:"flags,omitempty"`
	Variables       map[string]string     `json:"variables,omitempty"`
	Party           []*PartyMember        `json:"party,omitempty"`
	Quests          []QuestProgress       `json:"quests,omitempty"`

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
}

// MarshalJSON serializes the state under the mutex so an autosave can't race a
// concurrent mutation from the oracle goroutine (which would otherwise risk a
// "concurrent map iteration and map write" panic).
func (s *SessionState) MarshalJSON() ([]byte, error) {
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
	if s.Log == nil {
		s.Log = &SessionLog{Entries: []LogEntry{}, MaxSize: 0}
	}
	if s.Conversation == nil {
		s.Conversation = &Conversation{Messages: []Message{}, MaxSize: 0}
	}
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
		Name:            name,
		VisitedRooms:    make(map[string]bool),
		KnownNPCs:       make(map[string]*NPCStatus),
		TriggeredEvents: make(map[string]bool),
		Flags:           make(map[string]bool),
		Variables:       make(map[string]string),
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
		if len(adv.Zones) > 0 {
			s.CurrentZone = adv.Zones[0].ID
			if len(adv.Zones[0].Rooms) > 0 {
				s.CurrentRoom = adv.Zones[0].Rooms[0].ID
				s.VisitedRooms[s.CurrentRoom] = true
			}
		}
	}
	return s
}

func (s *SessionState) touch() { s.UpdatedAt = time.Now() }

// SetLocation records the party's current zone and room, marking the room
// visited and logging the move.
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
// the state; the timeline is replayed separately via AppendLog.
func (s *SessionState) ImportStructured(src *SessionState) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.CurrentZone = src.CurrentZone
	s.CurrentRoom = src.CurrentRoom
	s.VisitedRooms = src.VisitedRooms
	s.KnownNPCs = src.KnownNPCs
	s.TriggeredEvents = src.TriggeredEvents
	s.Flags = src.Flags
	s.Variables = src.Variables
	s.Party = src.Party
	s.Quests = src.Quests
	s.Characters = src.Characters
	s.PC = src.PC
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
}

// NewSession binds state, adventure, and config into a runtime session.
func NewSession(state *SessionState, adv *Adventure, config *Config) *Session {
	return &Session{
		State:     state,
		Adventure: adv,
		Config:    config,
		StartedAt: time.Now(),
	}
}

// MarkModified flags the session dirty and touches the timestamp.
func (s *Session) MarkModified() {
	s.IsModified = true
	s.State.mu.Lock()
	s.State.touch()
	s.State.mu.Unlock()
}
