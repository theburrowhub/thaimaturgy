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
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"
	"sync"

	"github.com/theburrowhub/thaimaturgy/internal/dmbook"
	"github.com/theburrowhub/thaimaturgy/internal/domain"
	"github.com/theburrowhub/thaimaturgy/internal/engine"
	"github.com/theburrowhub/thaimaturgy/internal/providers"
	"github.com/theburrowhub/thaimaturgy/internal/storage"
	"github.com/theburrowhub/thaimaturgy/internal/tgbot"
)

// ErrCharacterConflict is returned by UpdateCharacter when the live character no
// longer matches the baseline the caller captured — a concurrent edit would be
// clobbered, so the caller should reload and re-apply.
var ErrCharacterConflict = errors.New("character changed since it was loaded")

// ErrPartyConflict is returned by PlanParty when the party changed while the AI
// plan was in flight, so applying the plan would overwrite newer edits.
var ErrPartyConflict = errors.New("party changed while the plan was being generated")

// ErrNameConflict is returned by UpdateCharacter when the edit would rename a
// member onto another member's name, which would make name-based addressing of
// the two ambiguous.
var ErrNameConflict = errors.New("another party member already uses that name")

// ErrNovelConflict is returned by SaveNovelText when the stored novel no longer
// matches the version the caller loaded — a concurrent edit (or a regeneration)
// would be clobbered, so the caller should reload and re-apply.
var ErrNovelConflict = errors.New("the novel changed since it was loaded")

// Service is the facade. It is safe for concurrent use.
type Service struct {
	store    *storage.Storage
	provider providers.Provider

	mu       sync.Mutex // guards config + the sessions registry
	config   *domain.Config
	sessions map[string]*OpenSession // by session name

	configMu   sync.Mutex  // serializes SaveConfig's persist+adopt as one op
	autosaveCh chan string // session names queued for background save

	nameMu    sync.Mutex             // guards nameLocks
	nameLocks map[string]*sync.Mutex // per-session-name lifecycle locks

	jobMu      sync.Mutex            // guards importJobs/novelJobs + jobSeq
	jobSeq     int                   // monotonic id source for async jobs
	importJobs map[string]*ImportJob // AI-import jobs by id (#70)
	novelJobs  map[string]*NovelJob  // novelization jobs by id (#71)

	novelMu sync.Mutex // serializes the read-modify-write of saved novels (#65)

	// hostMu serializes the Telegram host lifecycle across ALL sessions. The
	// server has a single Telegram bot token, and Telegram allows only one
	// getUpdates consumer per bot, so at most one session may host at a time.
	// hostName is the session currently hosting ("" if none). Lock ordering:
	// hostMu is always acquired before any OpenSession.opMu.
	hostMu   sync.Mutex
	hostName string
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

	// tg is the Telegram bot hosting this session (nil when not hosting), set and
	// cleared under opMu. While it is non-nil the turn drivers reject other
	// clients (ErrSessionHosted) so the bot is the sole driver of the Oracle.
	tg       *tgbot.Bot
	tgCancel context.CancelFunc

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
		nameLocks:  make(map[string]*sync.Mutex),
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

// Config returns a DETACHED copy of the active configuration, so a caller
// mutating it (the usual "read, tweak a field, SaveConfig" pattern) doesn't
// change the running service before persistence succeeds, and concurrent callers
// don't share a mutable object outside the service's locks.
func (s *Service) Config() *domain.Config {
	s.mu.Lock()
	defer s.mu.Unlock()
	cp := *s.config
	return &cp
}

// SaveConfig persists the configuration and only then adopts a copy of it, as one
// serialized operation — so two concurrent calls can't leave the running service
// and the disk disagreeing, and a failed persist leaves the active config
// unchanged.
func (s *Service) SaveConfig(cfg *domain.Config) error {
	s.configMu.Lock()
	defer s.configMu.Unlock()
	if err := s.store.SaveConfig(cfg); err != nil {
		return err
	}
	cp := *cfg
	s.mu.Lock()
	s.config = &cp
	s.mu.Unlock()
	return nil
}

// lockName returns a per-session-name lock (creating it on first use) and locks
// it, returning the unlock func. Lifecycle operations on a session name (resume,
// rename, delete) hold it so they can't interleave — e.g. a resume can't register
// a session that a concurrent delete is removing. Acquire this BEFORE s.mu.
func (s *Service) lockName(name string) func() {
	s.nameMu.Lock()
	lk := s.nameLocks[name]
	if lk == nil {
		lk = &sync.Mutex{}
		s.nameLocks[name] = lk
	}
	s.nameMu.Unlock()
	lk.Lock()
	return lk.Unlock
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

// AdventureExists reports whether an adventure with the given ID is imported, so
// a transport can tell "not found" (404) apart from an operational delete failure
// (5xx).
func (s *Service) AdventureExists(id string) bool { return s.store.AdventureExists(id) }

// SaveAdventure persists an edited adventure back to its module directory. It
// pins adv.ID to the folder id so the editor can't move/rename the module by
// changing the field. Full validation is a separate step (ValidateAdventure), so
// a work-in-progress with, say, a not-yet-uploaded image can still be saved —
// mirroring the desktop editor.
func (s *Service) SaveAdventure(id string, adv *domain.Adventure) error {
	if adv == nil {
		return fmt.Errorf("no adventure supplied")
	}
	if strings.TrimSpace(adv.Title) == "" {
		return fmt.Errorf("the adventure needs a title")
	}
	adv.ID = id
	return s.store.SaveAdventure(id, adv)
}

// ValidateAdventure runs full validation (required fields, referential integrity,
// referenced images present) against a candidate adventure and returns the errors
// as strings (empty when valid).
func (s *Service) ValidateAdventure(id string, adv *domain.Adventure) []string {
	imageExists := func(rel string) bool { _, err := s.store.ResolveImagePath(id, rel); return err == nil }
	errs := domain.ValidateAdventure(adv, imageExists)
	out := make([]string, 0, len(errs))
	for _, e := range errs {
		out = append(out, e.Error())
	}
	return out
}

// ExportModule packages an imported adventure into a temporary .tar.gz and
// returns its path; the caller serves and then removes it.
func (s *Service) ExportModule(id string) (string, error) {
	if !s.store.AdventureExists(id) {
		return "", fmt.Errorf("adventure not found: %s", id)
	}
	tmp, err := os.CreateTemp("", "thaim-export-*.tar.gz")
	if err != nil {
		return "", err
	}
	tmpPath := tmp.Name()
	tmp.Close()
	if err := storage.PackageModule(s.store.AdventureDir(id), tmpPath); err != nil {
		_ = os.Remove(tmpPath)
		return "", err
	}
	return tmpPath, nil
}

// DMBookMarkdown renders the deterministic DM book (no AI) for an adventure.
func (s *Service) DMBookMarkdown(id string) (string, *domain.Adventure, error) {
	adv, err := s.store.LoadAdventure(id)
	if err != nil {
		return "", nil, err
	}
	return dmbook.Markdown(adv), adv, nil
}

// AdventureAsset resolves a module-relative image path to its absolute on-disk
// path, verifying it stays within the adventure's directory (path-traversal
// safe). It lets a transport serve module images without reaching into storage.
func (s *Service) AdventureAsset(id, relPath string) (string, error) {
	return s.store.ResolveImagePath(id, relPath)
}

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
	// Try candidate names in sequence; hold that name's lifecycle lock while
	// checking availability and registering, so a concurrent create/resume/rename
	// targeting the same name can't collide (lock order is always name → s.mu).
	for i := 0; ; i++ {
		name := adventureID
		if i > 0 {
			name = fmt.Sprintf("%s-%d", adventureID, i)
		}
		unlock := s.lockName(name)
		s.mu.Lock()
		if s.takenLocked(name) {
			s.mu.Unlock()
			unlock()
			continue
		}
		state := domain.NewSessionState(name, adv)
		_, regErr := s.registerLocked(state, adv)
		s.mu.Unlock()
		unlock()
		if regErr != nil {
			return "", regErr
		}
		return name, nil
	}
}

// ResumeSession loads a persisted session (and its adventure) and registers it
// live. Returns the OpenSession. Re-resuming an already-open session returns the
// existing one.
func (s *Service) ResumeSession(name string) (*OpenSession, error) {
	unlock := s.lockName(name) // serialize with rename/delete of this name
	defer unlock()
	if os, ok := s.Get(name); ok {
		return os, nil
	}
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
	unlock := s.lockName(name) // serialize with resume/rename/delete of this name
	defer unlock()
	// Stop any Telegram host for this session first (fully: cancel + wait for the
	// bot). Hold hostMu across the ENTIRE close so a concurrent StartTelegramHost
	// can't re-host in the gap before the session is marked closed (a later start
	// then sees closed and bails). Lock order is hostMu → opMu.
	s.hostMu.Lock()
	defer s.hostMu.Unlock()
	s.stopHostLocked(name)
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

// RenameSession renames a session on disk. It serializes on BOTH names (locked in
// a deterministic order to avoid deadlock) and rejects the rename if the source
// is open or the destination is open or already exists — so a rename can't
// collide with a concurrent resume/create/rename of either name, and can never
// overwrite another session (including a not-yet-saved open one, which has no
// file yet but is registered).
func (s *Service) RenameSession(oldName, newName string) error {
	first, second := oldName, newName
	if first > second {
		first, second = second, first
	}
	u1 := s.lockName(first)
	defer u1()
	if first != second {
		u2 := s.lockName(second)
		defer u2()
	}
	// A hosted session is always open, so the open-check below already refuses to
	// rename it; but the Telegram host is keyed by the (mutable) session name, so
	// make the invariant explicit here — renaming out from under a live host would
	// orphan its receive loop and leave hostName dangling. Lock order: lockName →
	// hostMu (consistent with CloseSession).
	s.hostMu.Lock()
	hosted := s.hostName == oldName
	s.hostMu.Unlock()
	if hosted {
		return fmt.Errorf("stop hosting %q on Telegram before renaming it", oldName)
	}
	if _, ok := s.Get(oldName); ok {
		return fmt.Errorf("close session %q before renaming it", oldName)
	}
	if _, ok := s.Get(newName); ok {
		return fmt.Errorf("a session named %q is open; close it before renaming onto it", newName)
	}
	// store.RenameSession additionally rejects a destination that exists on disk.
	return s.store.RenameSession(oldName, newName)
}

// DeleteSession removes a session; it must not be open. Serialized on the name so
// it can't race a concurrent resume that would otherwise re-register the loaded
// state and recreate the deleted session.
func (s *Service) DeleteSession(name string) error {
	unlock := s.lockName(name)
	defer unlock()
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
	if os.tg != nil {
		os.opMu.Unlock()
		return nil, ErrSessionHosted
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
	if os.tg != nil {
		os.opMu.Unlock()
		return nil, ErrSessionHosted
	}
	resp := os.Oracle.Ask(ctx, input)
	os.opMu.Unlock()
	s.Autosave(name)
	return resp, nil
}

// --- Party & characters (#67) --------------------------------------------

// withOpenSession runs fn under an open session's operation lock, rejecting a
// closed/unknown session, and autosaves afterwards when fn reports it mutated.
func (s *Service) withOpenSession(name string, fn func(os *OpenSession) (mutated bool, err error)) error {
	os, ok := s.Get(name)
	if !ok {
		return fmt.Errorf("session %q is not open", name)
	}
	os.opMu.Lock()
	if os.closed {
		os.opMu.Unlock()
		return fmt.Errorf("session %q is not open", name)
	}
	if os.tg != nil {
		os.opMu.Unlock()
		return ErrSessionHosted
	}
	mutated, err := fn(os)
	os.opMu.Unlock()
	if mutated {
		s.Autosave(name)
	}
	return err
}

// Party returns a snapshot of an open session's party.
func (s *Service) Party(name string) ([]domain.Character, error) {
	os, ok := s.Get(name)
	if !ok {
		return nil, fmt.Errorf("session %q is not open", name)
	}
	return os.Session.State.PartySnapshot(), nil
}

// SetParty replaces an open session's party.
func (s *Service) SetParty(name string, party []*domain.Character) error {
	return s.withOpenSession(name, func(os *OpenSession) (bool, error) {
		os.Session.State.SetParty(party)
		return true, nil
	})
}

// DefaultParty sets an open session's party to the built-in sample party.
func (s *Service) DefaultParty(name string) error {
	return s.SetParty(name, domain.DefaultParty())
}

// PlanParty asks the AI to build or update the party from a natural-language
// prompt, then applies the result — but only if the party hasn't changed since
// the (long) AI call started, so a concurrent edit isn't clobbered. It returns
// ErrPartyConflict otherwise.
func (s *Service) PlanParty(ctx context.Context, name, prompt string) ([]domain.Character, error) {
	os, ok := s.Get(name)
	if !ok {
		return nil, fmt.Errorf("session %q is not open", name)
	}
	s.mu.Lock()
	prov, model := s.provider, s.config.Model
	s.mu.Unlock()
	baseline := os.Session.State.PartySnapshot()
	baseJSON, _ := json.Marshal(baseline)
	party, err := engine.PlanParty(ctx, prov, model, prompt, baseline)
	if err != nil {
		return nil, err
	}
	var result []domain.Character
	applyErr := s.withOpenSession(name, func(os *OpenSession) (bool, error) {
		if cur, _ := json.Marshal(os.Session.State.PartySnapshot()); !bytes.Equal(cur, baseJSON) {
			return false, ErrPartyConflict
		}
		os.Session.State.SetParty(party)
		result = os.Session.State.PartySnapshot()
		return true, nil
	})
	if applyErr != nil {
		return nil, applyErr
	}
	return result, nil
}

// UpdateCharacter applies edited over the named session character, but only if
// the live record still matches base (optimistic concurrency); it returns
// ErrCharacterConflict otherwise. The character's ID is preserved.
func (s *Service) UpdateCharacter(name, charName string, base, edited *domain.Character) error {
	conflict := false
	found := false
	nameConflict := false
	err := s.withOpenSession(name, func(os *OpenSession) (bool, error) {
		// Reject a rename that collides with a DIFFERENT member's name, so that
		// name-based addressing stays unambiguous. The whole op runs under opMu, so
		// the snapshot can't race the mutation below.
		newName := strings.TrimSpace(edited.Name)
		for _, m := range os.Session.State.PartySnapshot() {
			if strings.EqualFold(m.Name, charName) {
				continue // the member being edited
			}
			if strings.EqualFold(m.Name, newName) {
				nameConflict = true
				return false, nil
			}
		}
		baseJSON, _ := json.Marshal(base)
		_, ok := os.Session.State.MutateCharacter(charName, func(c *domain.Character) {
			if cur, _ := json.Marshal(c); !bytes.Equal(cur, baseJSON) {
				conflict = true
				return
			}
			id := c.ID
			*c = *edited
			c.ID = id
			c.Normalize()
		})
		found = ok
		return ok && !conflict, nil
	})
	if err != nil {
		return err
	}
	if nameConflict {
		return ErrNameConflict
	}
	if !found {
		return fmt.Errorf("no character named %q in session %q", charName, name)
	}
	if conflict {
		return ErrCharacterConflict
	}
	return nil
}

// SavePartyToRoster saves each party member to the campaign roster and links the
// assigned IDs back into the session by position (not name, so duplicate names
// can't cross-link). On a partial failure it still links and persists the writes
// that succeeded — so no successful roster write is left unrecorded — and returns
// an error naming the member that failed.
func (s *Service) SavePartyToRoster(name string) error {
	var saveErr error
	err := s.withOpenSession(name, func(os *OpenSession) (bool, error) {
		snap := os.Session.State.PartySnapshot()
		ids := make([]string, len(snap))
		saved := 0
		for i := range snap {
			c := snap[i] // value copy; SaveCharacter assigns/returns its ID
			id, e := s.store.SaveCharacter(&c)
			if e != nil {
				saveErr = fmt.Errorf("saving %q to roster: %w", snap[i].Name, e)
				break
			}
			ids[i] = id
			saved++
		}
		// Link back whatever succeeded (by position) and persist synchronously, so
		// the ID links are durable even on a partial failure. We persist here rather
		// than returning mutated=true so the write ordering is explicit.
		if saved > 0 {
			os.Session.State.LinkRosterIDs(ids)
			if e := s.persist(os); e != nil && saveErr == nil {
				saveErr = e
			}
		}
		return false, nil
	})
	if err != nil {
		return err
	}
	return saveErr
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
