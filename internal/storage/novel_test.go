package storage

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/theburrowhub/thaimaturgy/internal/domain"
)

func TestNovelSaveLoadDelete(t *testing.T) {
	s := newTestStorage(t)
	if s.NovelExists("sess") {
		t.Error("a fresh session should have no novel")
	}
	if _, err := s.LoadNovel("sess"); !errors.Is(err, os.ErrNotExist) {
		t.Errorf("LoadNovel of a missing novel = %v; want os.ErrNotExist", err)
	}

	const md = "# Book\n\n## Chapter One\nProse."
	if err := s.SaveNovel("sess", md); err != nil {
		t.Fatalf("SaveNovel: %v", err)
	}
	if !s.NovelExists("sess") {
		t.Error("NovelExists should be true after save")
	}
	got, err := s.LoadNovel("sess")
	if err != nil || got != md {
		t.Fatalf("LoadNovel = %q (%v); want the saved markdown", got, err)
	}

	// Overwrite replaces content atomically.
	if err := s.SaveNovel("sess", "new text"); err != nil {
		t.Fatalf("SaveNovel overwrite: %v", err)
	}
	if got, _ := s.LoadNovel("sess"); got != "new text" {
		t.Errorf("overwrite failed, got %q", got)
	}

	if err := s.DeleteNovel("sess"); err != nil {
		t.Fatalf("DeleteNovel: %v", err)
	}
	if s.NovelExists("sess") {
		t.Error("novel should be gone after delete")
	}
	// Deleting a missing novel is not an error.
	if err := s.DeleteNovel("sess"); err != nil {
		t.Errorf("DeleteNovel of a missing novel should be nil, got %v", err)
	}
}

// Session names that could escape the sessions directory must be rejected, and
// nothing may be written or read outside it.
func TestNovelPathTraversalRejected(t *testing.T) {
	s := newTestStorage(t)
	bad := []string{"../evil", "../../etc/passwd", "sub/dir", `back\slash`, "", "a/../b", ".."}
	for _, name := range bad {
		if err := s.SaveNovel(name, "x"); err == nil {
			t.Errorf("SaveNovel(%q) should be rejected", name)
		}
		if _, err := s.LoadNovel(name); err == nil {
			t.Errorf("LoadNovel(%q) should be rejected", name)
		}
		if err := s.DeleteNovel(name); err == nil {
			t.Errorf("DeleteNovel(%q) should be rejected", name)
		}
		if s.NovelExists(name) {
			t.Errorf("NovelExists(%q) should be false", name)
		}
	}
	// A traversal attempt must not have created a file anywhere.
	if _, err := os.Stat(filepath.Join(s.BasePath(), "evil.novel.md")); err == nil {
		t.Error("a traversal name wrote a file outside the sessions dir")
	}
}

// RenameSession must carry the novel sibling file along with the session.
func TestRenameSessionMovesNovel(t *testing.T) {
	s := newTestStorage(t)
	st := domain.NewSessionState("old", &domain.Adventure{ID: "a", Title: "A"})
	if err := s.SaveSession(st); err != nil {
		t.Fatalf("SaveSession: %v", err)
	}
	if err := s.SaveNovel("old", "the novel"); err != nil {
		t.Fatalf("SaveNovel: %v", err)
	}
	if err := s.RenameSession("old", "new"); err != nil {
		t.Fatalf("RenameSession: %v", err)
	}
	if s.NovelExists("old") {
		t.Error("the old novel file should have been moved")
	}
	got, err := s.LoadNovel("new")
	if err != nil || got != "the novel" {
		t.Errorf("novel not moved to new name: %q (%v)", got, err)
	}
}

// DeleteSession must remove the novel so a later session reusing the name can't
// inherit stale prose.
func TestDeleteSessionRemovesNovel(t *testing.T) {
	s := newTestStorage(t)
	st := domain.NewSessionState("gone", &domain.Adventure{ID: "a", Title: "A"})
	if err := s.SaveSession(st); err != nil {
		t.Fatalf("SaveSession: %v", err)
	}
	if err := s.SaveNovel("gone", "prose"); err != nil {
		t.Fatalf("SaveNovel: %v", err)
	}
	if err := s.DeleteSession("gone"); err != nil {
		t.Fatalf("DeleteSession: %v", err)
	}
	if s.NovelExists("gone") {
		t.Error("the novel should be deleted with the session")
	}
}
