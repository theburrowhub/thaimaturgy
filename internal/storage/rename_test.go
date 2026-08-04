package storage

import (
	"os"
	"testing"

	"github.com/theburrowhub/thaimaturgy/internal/domain"
)

func TestRenameSessionMovesFileAndJournal(t *testing.T) {
	tmpDir, _ := os.MkdirTemp("", "thaimaturgy-rename-*")
	defer os.RemoveAll(tmpDir)
	store, _ := NewWithPath(tmpDir)

	state := domain.NewSessionState("run1", sampleAdventure())
	if err := store.SaveSession(state); err != nil {
		t.Fatalf("SaveSession: %v", err)
	}
	// Create an associated journal to ensure it moves with the session.
	j, err := store.OpenSessionJournal("run1")
	if err != nil {
		t.Fatalf("OpenSessionJournal: %v", err)
	}
	j.Note("test", "hello")
	_ = j.Close()

	if err := store.RenameSession("run1", "crypt-run1"); err != nil {
		t.Fatalf("RenameSession: %v", err)
	}

	if store.SessionExists("run1") {
		t.Error("old session should no longer exist")
	}
	if !store.SessionExists("crypt-run1") {
		t.Error("renamed session should exist")
	}
	loaded, err := store.LoadSession("crypt-run1")
	if err != nil {
		t.Fatalf("LoadSession(new): %v", err)
	}
	if loaded.Name != "crypt-run1" {
		t.Errorf("stored name = %q, want crypt-run1", loaded.Name)
	}
	// Journal moved with it; the old one is gone.
	if _, err := os.Stat(store.sessionJournalPath("run1")); !os.IsNotExist(err) {
		t.Error("old journal should have been moved")
	}
	if _, err := os.Stat(store.sessionJournalPath("crypt-run1")); err != nil {
		t.Errorf("new journal should exist: %v", err)
	}
}

func TestRenameSessionRejectsBadNames(t *testing.T) {
	tmpDir, _ := os.MkdirTemp("", "thaimaturgy-rename2-*")
	defer os.RemoveAll(tmpDir)
	store, _ := NewWithPath(tmpDir)

	_ = store.SaveSession(domain.NewSessionState("a", sampleAdventure()))
	_ = store.SaveSession(domain.NewSessionState("b", sampleAdventure()))

	if err := store.RenameSession("a", ""); err == nil {
		t.Error("empty new name should error")
	}
	if err := store.RenameSession("a", "../evil"); err == nil {
		t.Error("path-traversal name should error")
	}
	if err := store.RenameSession("a", "b"); err == nil {
		t.Error("renaming onto an existing session should error")
	}
}
