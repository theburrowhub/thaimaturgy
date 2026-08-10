package storage

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/theburrowhub/thaimaturgy/internal/domain"
)

func newTestStorage(t *testing.T) *Storage {
	t.Helper()
	s, err := NewWithPath(t.TempDir())
	if err != nil {
		t.Fatalf("NewWithPath: %v", err)
	}
	return s
}

func TestRosterSaveLoadListDelete(t *testing.T) {
	s := newTestStorage(t)

	c := domain.GenerateCharacter("Alice", "Elf", "Wizard", 3)
	id, err := s.SaveCharacter(c)
	if err != nil {
		t.Fatalf("SaveCharacter: %v", err)
	}
	if !strings.HasPrefix(id, "alice-") {
		t.Errorf("id = %q; want an 'alice-' prefixed id", id)
	}
	if c.ID != id {
		t.Errorf("SaveCharacter should set c.ID (got %q)", c.ID)
	}

	loaded, err := s.LoadCharacter(id)
	if err != nil {
		t.Fatalf("LoadCharacter: %v", err)
	}
	if loaded.Name != "Alice" || loaded.Level != 3 || loaded.ID != id {
		t.Errorf("loaded character wrong: %+v", loaded)
	}

	list, err := s.ListCharacters()
	if err != nil || len(list) != 1 {
		t.Fatalf("ListCharacters = %d (%v); want 1", len(list), err)
	}

	if err := s.DeleteCharacter(id); err != nil {
		t.Fatalf("DeleteCharacter: %v", err)
	}
	if list, _ := s.ListCharacters(); len(list) != 0 {
		t.Errorf("roster should be empty after delete, got %d", len(list))
	}
}

func TestRosterUniqueIDs(t *testing.T) {
	s := newTestStorage(t)
	a := domain.NewCharacter("Bob", "Human", "Fighter")
	b := domain.NewCharacter("Bob", "Dwarf", "Cleric")
	idA, _ := s.SaveCharacter(a)
	idB, _ := s.SaveCharacter(b)
	if idA == idB {
		t.Errorf("two 'Bob's must get distinct ids, both %q", idA)
	}
	if !strings.HasPrefix(idA, "bob-") || !strings.HasPrefix(idB, "bob-") {
		t.Errorf("ids should keep a readable 'bob-' prefix: %q, %q", idA, idB)
	}
	if list, _ := s.ListCharacters(); len(list) != 2 {
		t.Errorf("expected 2 distinct roster entries, got %d", len(list))
	}
}

func TestRosterIDsNonReusableAfterDelete(t *testing.T) {
	s := newTestStorage(t)
	first := domain.NewCharacter("Alice", "Elf", "Wizard")
	id1, _ := s.SaveCharacter(first)
	if err := s.DeleteCharacter(id1); err != nil {
		t.Fatalf("delete: %v", err)
	}
	replacement := domain.NewCharacter("Alice", "Human", "Rogue")
	id2, _ := s.SaveCharacter(replacement)
	if id1 == id2 {
		t.Fatalf("a recreated 'Alice' must get a NEW id, not reuse %q", id1)
	}
	// A stale session link to the deleted id must not overwrite the replacement.
	stale := domain.NewCharacter("Alice", "Elf", "Wizard")
	stale.ID = id1
	stale.XP = 9999
	n, err := s.SyncPartyToRoster([]*domain.Character{stale})
	if err != nil {
		t.Fatalf("sync: %v", err)
	}
	if n != 0 {
		t.Errorf("stale-id member must not be written back, synced %d", n)
	}
	if repl, _ := s.LoadCharacter(id2); repl.XP == 9999 {
		t.Error("stale link corrupted the replacement character")
	}
}

func TestSaveCharacterUpdatesInPlace(t *testing.T) {
	s := newTestStorage(t)
	c := domain.NewCharacter("Carol", "Human", "Rogue")
	id, _ := s.SaveCharacter(c)
	c.XP = 500 // progression
	if _, err := s.SaveCharacter(c); err != nil {
		t.Fatalf("re-save: %v", err)
	}
	if list, _ := s.ListCharacters(); len(list) != 1 {
		t.Errorf("re-saving the same id must update in place, got %d entries", len(list))
	}
	loaded, _ := s.LoadCharacter(id)
	if loaded.XP != 500 {
		t.Errorf("progression not persisted: XP=%d", loaded.XP)
	}
}

func TestSyncPartyToRosterOnlyLinked(t *testing.T) {
	s := newTestStorage(t)
	linked := domain.NewCharacter("Dana", "Elf", "Ranger")
	id, _ := s.SaveCharacter(linked)
	linked.XP = 1000

	adhoc := domain.NewCharacter("Random NPC", "Human", "Bard") // no ID → not roster-linked

	n, err := s.SyncPartyToRoster([]*domain.Character{linked, adhoc})
	if err != nil {
		t.Fatalf("SyncPartyToRoster: %v", err)
	}
	if n != 1 {
		t.Errorf("synced %d; want only the 1 linked member", n)
	}
	if loaded, _ := s.LoadCharacter(id); loaded.XP != 1000 {
		t.Errorf("linked progression not written back: XP=%d", loaded.XP)
	}
	if list, _ := s.ListCharacters(); len(list) != 1 {
		t.Errorf("ad-hoc member must not create a roster entry, got %d", len(list))
	}
}

func TestListCharactersReportsUnreadable(t *testing.T) {
	s := newTestStorage(t)
	good := domain.NewCharacter("Good", "Human", "Fighter")
	if _, err := s.SaveCharacter(good); err != nil {
		t.Fatalf("save good: %v", err)
	}
	// Drop a corrupt file directly into the roster dir.
	if err := os.WriteFile(filepath.Join(s.charactersDir(), "broken.json"), []byte("{not json"), 0644); err != nil {
		t.Fatalf("write corrupt: %v", err)
	}
	list, err := s.ListCharacters()
	if err == nil {
		t.Error("ListCharacters should report the unreadable entry")
	}
	if len(list) != 1 || list[0].Name != "Good" {
		t.Errorf("decoded characters should still be returned alongside the error, got %+v", list)
	}
	if err != nil && !strings.Contains(err.Error(), "broken") {
		t.Errorf("error should name the failing entry, got %v", err)
	}
}

func TestSaveCharacterAtomicNoTempLeftover(t *testing.T) {
	s := newTestStorage(t)
	c := domain.NewCharacter("Eve", "Human", "Bard")
	if _, err := s.SaveCharacter(c); err != nil {
		t.Fatalf("save: %v", err)
	}
	entries, _ := os.ReadDir(s.charactersDir())
	for _, e := range entries {
		if !strings.HasSuffix(e.Name(), ".json") || strings.HasPrefix(e.Name(), ".tmp-") {
			t.Errorf("atomic write left an unexpected file: %s", e.Name())
		}
	}
	// The list must still see exactly the one character.
	if list, _ := s.ListCharacters(); len(list) != 1 {
		t.Errorf("expected 1 character, got %d", len(list))
	}
}

func TestRosterRejectsUnsafeID(t *testing.T) {
	s := newTestStorage(t)
	if _, err := s.LoadCharacter("../secret"); err == nil {
		t.Error("path traversal id must be rejected")
	}
	if err := s.DeleteCharacter("a/b"); err == nil {
		t.Error("id with a slash must be rejected")
	}
}
