package appservice

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/theburrowhub/thaimaturgy/internal/domain"
	"github.com/theburrowhub/thaimaturgy/internal/rules"
	"github.com/theburrowhub/thaimaturgy/internal/storage"
)

// newService builds a Service over a fresh temp storage and writes one minimal
// adventure module to disk so session lifecycle can be exercised.
func newService(t *testing.T) (*Service, *storage.Storage) {
	t.Helper()
	store, err := storage.NewWithPath(t.TempDir())
	if err != nil {
		t.Fatalf("storage: %v", err)
	}
	adv := &domain.Adventure{
		SchemaVersion: domain.SchemaVersion, ID: "crypt", Title: "The Crypt",
		Zones: []domain.Zone{{ID: "z1", Name: "Entrance", Rooms: []domain.Room{{ID: "r1", Name: "Gate"}}}},
	}
	dir := store.AdventureDir("crypt")
	if err := os.MkdirAll(dir, 0755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	data, _ := json.MarshalIndent(adv, "", "  ")
	if err := os.WriteFile(filepath.Join(dir, storage.AdventureFile), data, 0644); err != nil {
		t.Fatalf("write adventure: %v", err)
	}
	service, err := New(store, domain.DefaultConfig(), nil)
	if err != nil {
		t.Fatalf("service: %v", err)
	}
	t.Cleanup(service.Close)
	return service, store
}

func TestSessionLifecycle(t *testing.T) {
	svc, _ := newService(t)

	advs, err := svc.ListAdventures()
	if err != nil || len(advs) != 1 || advs[0].ID != "crypt" {
		t.Fatalf("ListAdventures = %+v (%v)", advs, err)
	}

	name, err := svc.NewSession("crypt")
	if err != nil {
		t.Fatalf("NewSession: %v", err)
	}
	if name != "crypt" {
		t.Errorf("first session name = %q; want crypt", name)
	}
	opened, ok := svc.Get(name)
	if !ok {
		t.Error("new session should be registered/open")
	}
	if opened.Session.PersistRules == nil || opened.Session.RulesResolver == nil {
		t.Fatal("live session is missing its durable rules runtime")
	}
	if snapshot, ok := opened.Session.State.RulesSnapshot(); !ok || snapshot.Ruleset.ID != "dnd5e" {
		t.Fatalf("fresh session rules snapshot = %+v, ok=%v", snapshot, ok)
	}

	// A second new session for the same adventure gets a distinct name.
	name2, err := svc.NewSession("crypt")
	if err != nil || name2 == name {
		t.Fatalf("second NewSession = %q (%v); want a distinct name", name2, err)
	}

	// Persist and confirm it lists.
	if err := svc.SaveSession(name); err != nil {
		t.Fatalf("SaveSession: %v", err)
	}
	sessions, _ := svc.ListSessions()
	if len(sessions) < 1 {
		t.Errorf("expected saved sessions, got %d", len(sessions))
	}

	// Close, then resume from disk.
	if err := svc.CloseSession(name); err != nil {
		t.Fatalf("CloseSession: %v", err)
	}
	if _, ok := svc.Get(name); ok {
		t.Error("closed session should be unregistered")
	}
	os, err := svc.ResumeSession(name)
	if err != nil || os == nil {
		t.Fatalf("ResumeSession: %v", err)
	}
	if os.Session.Adventure.ID != "crypt" {
		t.Errorf("resumed adventure = %q; want crypt", os.Session.Adventure.ID)
	}
}

func TestForeignRulesSessionRejectsLegacyDNDUtilities(t *testing.T) {
	svc, store := newService(t)
	adv := &domain.Adventure{
		SchemaVersion: domain.SchemaVersion,
		ID:            "pbta-game",
		Title:         "PbtA Game",
		System:        "PbtA",
		Ruleset:       &rules.Requirement{ID: "pbta", Version: "0.1.0"},
		Zones:         []domain.Zone{{ID: "z", Name: "Zone", Rooms: []domain.Room{{ID: "r", Name: "Room"}}}},
	}
	dir := store.AdventureDir(adv.ID)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	data, err := json.MarshalIndent(adv, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, storage.AdventureFile), data, 0o644); err != nil {
		t.Fatal(err)
	}
	name, err := svc.NewSession(adv.ID)
	if err != nil {
		t.Fatalf("NewSession: %v", err)
	}
	capabilities, err := svc.Capabilities(name)
	if err != nil || capabilities.LegacyDND5E {
		t.Fatalf("Capabilities = %+v, %v", capabilities, err)
	}

	checks := []struct {
		name string
		run  func() error
	}{
		{"read party", func() error { _, err := svc.Party(name); return err }},
		{"set party", func() error { return svc.SetParty(name, domain.DefaultParty()) }},
		{"default party", func() error { return svc.DefaultParty(name) }},
		{"plan party", func() error { _, err := svc.PlanParty(context.Background(), name, "heroes"); return err }},
		{"update character", func() error { return svc.UpdateCharacter(name, "x", &domain.Character{}, &domain.Character{}) }},
		{"save party", func() error { return svc.SavePartyToRoster(name) }},
		{"telegram", func() error { _, err := svc.StartTelegramHost(name); return err }},
	}
	for _, check := range checks {
		t.Run(check.name, func(t *testing.T) {
			if err := check.run(); !errors.Is(err, ErrDNDUtilitiesUnavailable) {
				t.Fatalf("error = %v; want ErrDNDUtilitiesUnavailable", err)
			}
		})
	}
	opened, _ := svc.Get(name)
	if party := opened.Session.State.PartySnapshot(); len(party) != 0 {
		t.Fatalf("foreign legacy party mutated: %+v", party)
	}
}

func TestResumeSessionMissingExactRulesArtifactFailsWithoutRegistration(t *testing.T) {
	svc, store := newService(t)
	adventure, err := store.LoadAdventure("crypt")
	if err != nil {
		t.Fatal(err)
	}
	state := domain.NewSessionState("missing-rules", adventure)
	empty, err := rules.PayloadFrom(map[string]any{})
	if err != nil {
		t.Fatal(err)
	}
	missing := rules.Lock{
		ID: "missing.rules", Version: "1.0.0", ProtocolVersion: rules.ProtocolVersion,
		Digest: "sha256:" + strings.Repeat("a", 64),
	}
	if created, err := state.BindRules(missing, empty); err != nil || !created {
		t.Fatalf("bind missing lock created=%v err=%v", created, err)
	}
	if err := store.SaveSession(state); err != nil {
		t.Fatal(err)
	}

	if opened, err := svc.ResumeSession(state.Name); err == nil || opened != nil || !strings.Contains(err.Error(), "exact session lock") {
		t.Fatalf("ResumeSession opened=%v err=%v", opened, err)
	}
	if _, ok := svc.Get(state.Name); ok {
		t.Fatal("failed resume was registered")
	}
	reloaded, err := store.LoadSession(state.Name)
	if err != nil {
		t.Fatal(err)
	}
	snapshot, ok := reloaded.RulesSnapshot()
	if !ok || snapshot.Ruleset != missing || snapshot.Revision != 0 {
		t.Fatalf("failed resume mutated persisted rules: %+v ok=%v", snapshot, ok)
	}
}

func TestExecuteCommandAndAutosave(t *testing.T) {
	svc, store := newService(t)
	name, err := svc.NewSession("crypt")
	if err != nil {
		t.Fatalf("NewSession: %v", err)
	}
	// A note command mutates the session and should succeed.
	res, err := svc.ExecuteCommand(name, "/note the door is ajar")
	if err != nil {
		t.Fatalf("ExecuteCommand: %v", err)
	}
	if res == nil || !res.Success {
		t.Fatalf("command result = %+v; want success", res)
	}
	// Autosave is async; force a synchronous save and reload to confirm the note
	// persisted through the facade path.
	if err := svc.SaveSession(name); err != nil {
		t.Fatalf("save: %v", err)
	}
	reloaded, err := store.LoadSession(name)
	if err != nil {
		t.Fatalf("reload: %v", err)
	}
	found := false
	for _, e := range reloaded.Log.Entries {
		if e.Type == domain.LogNote && e.Message == "the door is ajar" {
			found = true
		}
	}
	if !found {
		t.Error("the note did not persist through the facade")
	}
}

func TestServiceCloseDrainsAutosavesAndIsIdempotent(t *testing.T) {
	svc, store := newService(t)
	name, err := svc.NewSession("crypt")
	if err != nil {
		t.Fatalf("NewSession: %v", err)
	}
	if _, err := svc.ExecuteCommand(name, "/note persisted before shutdown"); err != nil {
		t.Fatalf("ExecuteCommand: %v", err)
	}

	done := make(chan struct{})
	go func() {
		var wg sync.WaitGroup
		for range 4 {
			wg.Add(1)
			go func() {
				defer wg.Done()
				svc.Close()
			}()
		}
		wg.Wait()
		svc.Autosave(name) // shutdown turns later enqueue attempts into no-ops
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("Service.Close did not stop the autosave worker")
	}

	reloaded, err := store.LoadSession(name)
	if err != nil {
		t.Fatalf("LoadSession: %v", err)
	}
	for _, entry := range reloaded.Log.Entries {
		if entry.Type == domain.LogNote && entry.Message == "persisted before shutdown" {
			return
		}
	}
	t.Fatal("Close returned before the queued autosave was durable")
}

func TestNewSessionConcurrentUniqueNames(t *testing.T) {
	svc, _ := newService(t)
	const n = 8
	names := make(chan string, n)
	errs := make(chan error, n)
	var wg sync.WaitGroup
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			name, err := svc.NewSession("crypt")
			if err != nil {
				errs <- err
				return
			}
			names <- name
		}()
	}
	wg.Wait()
	close(names)
	close(errs)
	for err := range errs {
		t.Fatalf("concurrent NewSession: %v", err)
	}
	seen := map[string]bool{}
	count := 0
	for name := range names {
		if seen[name] {
			t.Errorf("duplicate session name handed out: %q", name)
		}
		seen[name] = true
		count++
	}
	if count != n {
		t.Errorf("expected %d sessions, got %d", n, count)
	}
}

func TestCloseSessionSaveFailureKeepsLive(t *testing.T) {
	svc, store := newService(t)
	name, err := svc.NewSession("crypt")
	if err != nil {
		t.Fatalf("NewSession: %v", err)
	}
	// Make the sessions directory unwritable so the final save fails.
	sessDir := filepath.Join(store.BasePath(), storage.SessionsDir)
	if err := os.Chmod(sessDir, 0o555); err != nil {
		t.Skipf("cannot chmod sessions dir (skipping): %v", err)
	}
	t.Cleanup(func() { _ = os.Chmod(sessDir, 0o755) })

	if err := svc.CloseSession(name); err == nil {
		t.Error("CloseSession should error when the final save fails")
	}
	if _, ok := svc.Get(name); !ok {
		t.Error("a session whose final save failed must stay live (retryable), not be discarded")
	}
	if svc.AutosaveError(name) == nil {
		t.Error("the save failure should be recorded and surfaced via AutosaveError")
	}
}

func TestExecuteCommandUnopenedSession(t *testing.T) {
	svc, _ := newService(t)
	if _, err := svc.ExecuteCommand("nope", "/note x"); err == nil {
		t.Error("executing against an unopened session should error")
	}
}

func TestExecuteCommandRejectedAfterClose(t *testing.T) {
	svc, _ := newService(t)
	name, _ := svc.NewSession("crypt")
	if err := svc.CloseSession(name); err != nil {
		t.Fatalf("close: %v", err)
	}
	if _, err := svc.ExecuteCommand(name, "/note late"); err == nil {
		t.Error("a command after close must be rejected, not silently lost")
	}
}

func TestCloseSessionWritesRosterBack(t *testing.T) {
	svc, store := newService(t)
	// A roster character that the party will control.
	id, err := store.SaveCharacter(domain.NewCharacter("Alice", "Elf", "Wizard"))
	if err != nil {
		t.Fatalf("save char: %v", err)
	}
	name, _ := svc.NewSession("crypt")
	os, _ := svc.Get(name)
	linked := domain.NewCharacter("Alice", "Elf", "Wizard")
	linked.ID = id
	linked.XP = 777 // progression made during the session
	os.Session.State.SetParty([]*domain.Character{linked})

	if err := svc.CloseSession(name); err != nil {
		t.Fatalf("close: %v", err)
	}
	// The final close save must have written the roster-linked progression back.
	reloaded, err := store.LoadCharacter(id)
	if err != nil {
		t.Fatalf("load char: %v", err)
	}
	if reloaded.XP != 777 {
		t.Errorf("roster progression not written back on close: XP=%d", reloaded.XP)
	}
}

func TestRosterAndConfigDelegation(t *testing.T) {
	svc, _ := newService(t)
	id, err := svc.SaveCharacter(domain.NewCharacter("Alice", "Elf", "Wizard"))
	if err != nil {
		t.Fatalf("SaveCharacter: %v", err)
	}
	list, err := svc.ListCharacters()
	if err != nil || len(list) != 1 {
		t.Fatalf("ListCharacters = %d (%v)", len(list), err)
	}
	if err := svc.DeleteCharacter(id); err != nil {
		t.Fatalf("DeleteCharacter: %v", err)
	}

	cfg := svc.Config()
	cfg.Language = "es"
	if err := svc.SaveConfig(cfg); err != nil {
		t.Fatalf("SaveConfig: %v", err)
	}
	if svc.Config().Language != "es" {
		t.Error("SaveConfig should adopt the new config")
	}
}

func TestResumeVsDeleteSerialized(t *testing.T) {
	svc, store := newService(t)
	name, _ := svc.NewSession("crypt")
	if err := svc.SaveSession(name); err != nil {
		t.Fatalf("save: %v", err)
	}
	if err := svc.CloseSession(name); err != nil {
		t.Fatalf("close: %v", err)
	}

	// Delete then resume: resume must fail (file gone), not recreate it.
	if err := svc.DeleteSession(name); err != nil {
		t.Fatalf("delete: %v", err)
	}
	if _, err := svc.ResumeSession(name); err == nil {
		t.Error("resuming a deleted session should fail, not recreate it")
	}
	if svc.store.SessionExists(name) {
		t.Error("a deleted session must not be recreated on disk")
	}
	_ = store

	// Resume then delete: delete must be refused while open.
	name2, _ := svc.NewSession("crypt")
	_ = svc.SaveSession(name2)
	_ = svc.CloseSession(name2)
	if _, err := svc.ResumeSession(name2); err != nil {
		t.Fatalf("resume: %v", err)
	}
	if err := svc.DeleteSession(name2); err == nil {
		t.Error("deleting an open (resumed) session should be refused")
	}
}

// TestResumeDeleteConcurrent stresses the per-name lock under the race detector:
// after the dust settles the session is either open (resumed) or gone (deleted),
// never "deleted on disk yet somehow re-registered".
func TestResumeDeleteConcurrent(t *testing.T) {
	svc, _ := newService(t)
	for i := 0; i < 25; i++ {
		name, _ := svc.NewSession("crypt")
		_ = svc.SaveSession(name)
		_ = svc.CloseSession(name)

		var wg sync.WaitGroup
		wg.Add(2)
		go func() { defer wg.Done(); _, _ = svc.ResumeSession(name) }()
		go func() { defer wg.Done(); _ = svc.DeleteSession(name) }()
		wg.Wait()

		_, open := svc.Get(name)
		onDisk := svc.store.SessionExists(name)
		if !open && onDisk {
			// resume lost, delete won → file must be gone
			t.Fatalf("inconsistent: deleted but still on disk (%s)", name)
		}
		if open && !onDisk {
			// resumed → a save must be able to persist it (it exists in memory)
			if err := svc.SaveSession(name); err != nil {
				t.Fatalf("resumed session not persistable: %v", err)
			}
		}
		_ = svc.CloseSession(name)
		_ = svc.DeleteSession(name)
	}
}

func TestRenameRejectsOpenDestination(t *testing.T) {
	svc, _ := newService(t)
	// name1 persisted and closed.
	name1, _ := svc.NewSession("crypt")
	_ = svc.SaveSession(name1)
	_ = svc.CloseSession(name1)
	// name2 open but not yet saved (no file on disk).
	name2, _ := svc.NewSession("crypt")
	if _, ok := svc.Get(name2); !ok {
		t.Fatalf("name2 %q should be open", name2)
	}
	// Renaming name1 onto the open name2 must be refused, not silently overwrite.
	if err := svc.RenameSession(name1, name2); err == nil {
		t.Error("renaming onto an open (not-yet-saved) destination must be refused")
	}
	// name1's data must still be intact under its original name.
	if !svc.store.SessionExists(name1) {
		t.Error("the source session must not have been moved/lost")
	}
}

func TestConfigReturnsDetachedCopy(t *testing.T) {
	svc, _ := newService(t)
	cfg := svc.Config()
	cfg.Language = "zz" // mutate the returned copy without saving
	if svc.Config().Language == "zz" {
		t.Error("Config() must return a detached copy; mutating it changed the active config")
	}
	if svc.Config() == svc.Config() {
		t.Error("Config() should hand out distinct copies, not the internal pointer")
	}
}

func TestRenameAndDeleteRequireClosed(t *testing.T) {
	svc, _ := newService(t)
	name, _ := svc.NewSession("crypt")
	if err := svc.RenameSession(name, "other"); err == nil {
		t.Error("renaming an open session should be refused")
	}
	if err := svc.DeleteSession(name); err == nil {
		t.Error("deleting an open session should be refused")
	}
	if err := svc.CloseSession(name); err != nil {
		t.Fatalf("close: %v", err)
	}
	if err := svc.DeleteSession(name); err != nil {
		t.Errorf("deleting a closed session should work: %v", err)
	}
}
