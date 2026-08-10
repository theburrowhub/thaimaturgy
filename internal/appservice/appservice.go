// Package appservice is the transport-agnostic service facade for thAImaturgy
// (issue #36, Phase A). It expresses the app's use-cases — the same operations
// the desktop GUI performs — over the UI-agnostic core (domain, storage, engine,
// providers), and owns a registry of open play sessions with server-side FIFO
// autosave.
//
// It is the seam the future HTTP/WS server (Phase B) and the desktop GUI (as an
// in-process or remote client) call, so business logic stays out of the
// transports. Adding this package changes no existing behavior; callers adopt it
// incrementally.
package appservice

import (
	"context"
	"fmt"
	"strings"
	"sync"

	"github.com/theburrowhub/thaimaturgy/internal/domain"
	"github.com/theburrowhub/thaimaturgy/internal/engine"
	"github.com/theburrowhub/thaimaturgy/internal/providers"
	"github.com/theburrowhub/thaimaturgy/internal/storage"
)

// Service is the facade. It is safe for concurrent use.
type Service struct {
	store    *storage.Storage
	provider providers.Provider

	mu       sync.Mutex // guards config + the sessions registry
	config   *domain.Config
	sessions map[string]*OpenSession // by session name

	configMu   sync.Mutex  // serializes SaveConfig's persist+adopt as one op
	autosaveCh chan string // session names queued for background save
}

// OpenSession is a live, registered play session with its engine bindings.
type OpenSession struct {
	Session *domain.Session
	Oracle  *engine.Oracle
	Cmd     *engine.CommandHandler
	journal *storage.SessionJournal

	// opMu serializes this session's mutating operations (command, oracle turn,
	// save) with its closure, so no mutation or save crosses CloseSession. closed
	// is set (under opMu) once the session has been closed, so an operation that
	// wins the lock after closure bails instead of acting on a dead session.
	opMu   sync.Mutex
	closed bool

	errMu       sync.Mutex
	lastSaveErr error // most recent save failure (nil once a save succeeds)
}

func (o *OpenSession) setSaveErr(err error) {
	o.errMu.Lock()
	o.lastSaveErr = err
	o.errMu.Unlock()
}

// SaveError returns the most recent autosave error for this session (nil if the
// last autosave succeeded).
func (o *OpenSession) SaveError() error {
	o.errMu.Lock()
	defer o.errMu.Unlock()
	return o.lastSaveErr
}

// New builds a service over a storage, config, and (optional) LLM provider, and
// starts the FIFO autosave worker. provider may be nil for non-oracle use.
func New(store *storage.Storage, config *domain.Config, provider providers.Provider) *Service {
	s := &Service{
		store:      store,
		provider:   provider,
		config:     config,
		sessions:   make(map[string]*OpenSession),
		autosaveCh: make(chan string, 128),
	}
	go s.autosaveLoop()
	return s
}

// persist saves a session and writes roster progression back (#33). The caller
// must hold the session's opMu. It records the outcome on the session for
// AutosaveError and returns it.
func (s *Service) persist(os *OpenSession) error {
	err := s.store.SaveSession(os.Session.State)
	if err == nil {
		_, err = s.store.SyncPartyToRoster(rosterLinked(os.Session.State))
	}
	os.setSaveErr(err)
	return err
}

// SetProvider swaps the active provider for future oracle turns (e.g. after a
// settings change).
func (s *Service) SetProvider(p providers.Provider) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.provider = p
	for _, os := range s.sessions {
		os.Oracle.SetProvider(p)
	}
}

// Config returns the active configuration.
func (s *Service) Config() *domain.Config {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.config
}

// SaveConfig persists the configuration and adopts it as one serialized
// operation, so two concurrent calls can't leave the running service and the disk
// disagreeing about which config won.
func (s *Service) SaveConfig(cfg *domain.Config) error {
	s.configMu.Lock()
	defer s.configMu.Unlock()
	if err := s.store.SaveConfig(cfg); err != nil {
		return err
	}
	s.mu.Lock()
	s.config = cfg
	s.mu.Unlock()
	return nil
}

// --- Adventures ----------------------------------------------------------

func (s *Service) ListAdventures() ([]storage.AdventureInfo, error) { return s.store.ListAdventures() }
func (s *Service) LoadAdventure(id string) (*domain.Adventure, error) {
	return s.store.LoadAdventure(id)
}
func (s *Service) ImportAdventure(path string) (*domain.Adventure, error) {
	return s.store.ImportModule(path)
}
func (s *Service) DeleteAdventure(id string) error { return s.store.DeleteAdventure(id) }

// --- Sessions ------------------------------------------------------------

func (s *Service) ListSessions() ([]storage.SessionInfo, error) { return s.store.ListSessions() }

// takenLocked reports whether a session name is already used — persisted on disk
// or open in memory. Caller holds s.mu.
func (s *Service) takenLocked(name string) bool {
	if _, ok := s.sessions[name]; ok {
		return true
	}
	return s.store.SessionExists(name)
}

// NewSession creates a fresh session for an adventure (choosing a unique name
// derived from the adventure id), registers it live, and returns its name. Name
// selection and registration happen under one lock, so two concurrent calls for
// the same adventure can't be handed the same name.
func (s *Service) NewSession(adventureID string) (string, error) {
	adv, err := s.store.LoadAdventure(adventureID)
	if err != nil {
		return "", err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	name := adventureID
	for i := 1; s.takenLocked(name); i++ {
		name = fmt.Sprintf("%s-%d", adventureID, i)
	}
	state := domain.NewSessionState(name, adv)
	if _, err := s.registerLocked(state, adv); err != nil {
		return "", err
	}
	return name, nil
}

// ResumeSession loads a persisted session (and its adventure) and registers it
// live. Returns the OpenSession. Re-resuming an already-open session returns the
// existing one.
func (s *Service) ResumeSession(name string) (*OpenSession, error) {
	s.mu.Lock()
	if os, ok := s.sessions[name]; ok {
		s.mu.Unlock()
		return os, nil
	}
	s.mu.Unlock()
	state, err := s.store.LoadSession(name)
	if err != nil {
		return nil, err
	}
	adv, err := s.store.LoadAdventure(state.AdventureID)
	if err != nil {
		return nil, fmt.Errorf("adventure %q not found; import it first", state.AdventureID)
	}
	return s.register(state, adv)
}

// register binds a state+adventure into a live session and registers it. Locks.
func (s *Service) register(state *domain.SessionState, adv *domain.Adventure) (*OpenSession, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.registerLocked(state, adv)
}

// registerLocked is the lock-free core of register (caller holds s.mu). It
// requires the session's journal to open — durability is a precondition — so it
// returns an error and registers nothing if the journal can't be created.
func (s *Service) registerLocked(state *domain.SessionState, adv *domain.Adventure) (*OpenSession, error) {
	if os, ok := s.sessions[state.Name]; ok {
		return os, nil
	}
	journal, err := s.store.OpenSessionJournal(state.Name)
	if err != nil {
		return nil, fmt.Errorf("cannot open session journal for %q: %w", state.Name, err)
	}
	state.SetLogHook(func(e domain.LogEntry) { journal.Append(e) })
	sess := domain.NewSession(state, adv, s.config)
	os := &OpenSession{
		Session: sess,
		Oracle:  engine.NewOracle(sess, s.provider),
		Cmd:     engine.NewCommandHandler(sess),
		journal: journal,
	}
	s.sessions[state.Name] = os
	return os, nil
}

// Get returns a live session by name, if open.
func (s *Service) Get(name string) (*OpenSession, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	os, ok := s.sessions[name]
	return os, ok
}

// SaveSession persists an open session synchronously, re-validating under the
// session's opMu that it's still open — so a save can't race its closure and
// resurrect a deleted/renamed session.
func (s *Service) SaveSession(name string) error {
	os, ok := s.Get(name)
	if !ok {
		return fmt.Errorf("session %q is not open", name)
	}
	os.opMu.Lock()
	defer os.opMu.Unlock()
	if os.closed {
		return fmt.Errorf("session %q is not open", name)
	}
	return s.persist(os)
}

// AutosaveError returns the most recent autosave error for an open session (nil
// if none / the last save succeeded), so a caller can surface a durable-write
// failure. Returns nil for an unknown session.
func (s *Service) AutosaveError(name string) error {
	if os, ok := s.Get(name); ok {
		return os.SaveError()
	}
	return nil
}

// CloseSession does a final save (including roster write-back) FIRST, and only
// then marks the session closed, unregisters it, and closes its journal — so a
// failed final save leaves the session live and retryable rather than discarding
// unsaved changes. It holds the session's opMu across the whole thing, so it
// waits for any in-flight command/oracle turn and blocks new ones (they see
// closed and bail), and its final save captures their mutations. Roster
// progression is written back here too, so a queued-but-dropped autosave can't
// leave the roster stale.
func (s *Service) CloseSession(name string) error {
	os, ok := s.Get(name)
	if !ok {
		return nil
	}
	os.opMu.Lock()
	defer os.opMu.Unlock()
	if os.closed {
		return nil
	}
	if err := s.persist(os); err != nil {
		return fmt.Errorf("not closing %q — final save failed (retry): %w", name, err)
	}
	os.closed = true
	s.mu.Lock()
	delete(s.sessions, name)
	s.mu.Unlock()
	if os.journal != nil {
		_ = os.journal.Close()
	}
	return nil
}

// RenameSession renames a session on disk; it must not be open.
func (s *Service) RenameSession(oldName, newName string) error {
	if _, ok := s.Get(oldName); ok {
		return fmt.Errorf("close session %q before renaming it", oldName)
	}
	return s.store.RenameSession(oldName, newName)
}

// DeleteSession removes a session; it must not be open.
func (s *Service) DeleteSession(name string) error {
	if _, ok := s.Get(name); ok {
		return fmt.Errorf("close session %q before deleting it", name)
	}
	return s.store.DeleteSession(name)
}

// ExecuteCommand runs a shared engine command (the parity path, #20) against an
// open session and returns its result, autosaving after a successful mutation.
func (s *Service) ExecuteCommand(name, raw string) (*engine.CommandResult, error) {
	os, ok := s.Get(name)
	if !ok {
		return nil, fmt.Errorf("session %q is not open", name)
	}
	os.opMu.Lock()
	if os.closed {
		os.opMu.Unlock()
		return nil, fmt.Errorf("session %q is not open", name)
	}
	res := os.Cmd.Execute(engine.ParseCommand(raw))
	os.opMu.Unlock()
	if res != nil && res.Success {
		s.Autosave(name)
	}
	return res, nil
}

// AskOracle runs one oracle/DM turn against an open session and returns the
// response, autosaving afterwards. The turn holds the session's opMu, so a close
// waits for it to finish and captures its mutations. Requires a configured
// provider.
func (s *Service) AskOracle(ctx context.Context, name, input string) (*engine.Response, error) {
	os, ok := s.Get(name)
	if !ok {
		return nil, fmt.Errorf("session %q is not open", name)
	}
	os.opMu.Lock()
	if os.closed {
		os.opMu.Unlock()
		return nil, fmt.Errorf("session %q is not open", name)
	}
	resp := os.Oracle.Ask(ctx, input)
	os.opMu.Unlock()
	s.Autosave(name)
	return resp, nil
}

// --- Roster (#33) --------------------------------------------------------

func (s *Service) ListCharacters() ([]*domain.Character, error) { return s.store.ListCharacters() }
func (s *Service) SaveCharacter(c *domain.Character) (string, error) {
	return s.store.SaveCharacter(c)
}
func (s *Service) DeleteCharacter(id string) error { return s.store.DeleteCharacter(id) }

// --- Autosave ------------------------------------------------------------

// Autosave enqueues an open session for a background save (FIFO by name, so saves
// commit in request order). No-op if the session isn't open.
func (s *Service) Autosave(name string) {
	if _, ok := s.Get(name); !ok {
		return
	}
	s.autosaveCh <- name
}

func (s *Service) autosaveLoop() {
	for name := range s.autosaveCh {
		s.doAutosave(name)
	}
}

// doAutosave saves a session by name under its opMu, skipping it if it closed
// meanwhile — so a save queued before CloseSession/DeleteSession can't resurrect
// the session or leave a stale duplicate (CloseSession already persisted the
// latest state before closing). Failures are recorded for AutosaveError.
func (s *Service) doAutosave(name string) {
	os, ok := s.Get(name)
	if !ok {
		return
	}
	os.opMu.Lock()
	defer os.opMu.Unlock()
	if os.closed {
		return // closed meanwhile → its final save already persisted the state
	}
	_ = s.persist(os)
}

func rosterLinked(st *domain.SessionState) []*domain.Character {
	snap := st.PartySnapshot()
	out := make([]*domain.Character, 0, len(snap))
	for i := range snap {
		if strings.TrimSpace(snap[i].ID) != "" {
			c := snap[i]
			out = append(out, &c)
		}
	}
	return out
}
