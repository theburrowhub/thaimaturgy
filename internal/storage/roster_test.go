package storage

import (
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
	if id != "alice" {
		t.Errorf("id = %q; want alice", id)
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
	if idA != "bob" || idB != "bob-2" {
		t.Errorf("ids = %q, %q; want bob, bob-2", idA, idB)
	}
	if list, _ := s.ListCharacters(); len(list) != 2 {
		t.Errorf("expected 2 distinct roster entries, got %d", len(list))
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
	deleted := domain.NewCharacter("Ghost", "Human", "Wizard")
	delID, _ := s.SaveCharacter(deleted)
	_ = s.DeleteCharacter(delID) // roster entry gone; must not be resurrected
	deleted.XP = 42

	n, err := s.SyncPartyToRoster([]*domain.Character{linked, adhoc, deleted})
	if err != nil {
		t.Fatalf("SyncPartyToRoster: %v", err)
	}
	if n != 1 {
		t.Errorf("synced %d; want only the 1 linked+existing member", n)
	}
	if loaded, _ := s.LoadCharacter(id); loaded.XP != 1000 {
		t.Errorf("linked progression not written back: XP=%d", loaded.XP)
	}
	if list, _ := s.ListCharacters(); len(list) != 1 {
		t.Errorf("ad-hoc/deleted members must not create roster entries, got %d", len(list))
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
