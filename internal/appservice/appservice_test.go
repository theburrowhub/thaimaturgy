package appservice

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/theburrowhub/thaimaturgy/internal/domain"
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
	return New(store, domain.DefaultConfig(), nil), store
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
	if _, ok := svc.Get(name); !ok {
		t.Error("new session should be registered/open")
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

func TestExecuteCommandUnopenedSession(t *testing.T) {
	svc, _ := newService(t)
	if _, err := svc.ExecuteCommand("nope", "/note x"); err == nil {
		t.Error("executing against an unopened session should error")
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
