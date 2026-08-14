package storage

import (
	"fmt"
	"os"
	"path/filepath"
)

// The novelization of a session is stored as a sibling Markdown file next to the
// session JSON (mirroring the journal at "<name>.journal.md"), so the possibly
// large prose stays out of the session file and can be re-opened and edited.
func (s *Storage) sessionNovelPath(name string) string {
	return filepath.Join(s.basePath, SessionsDir, name+".novel.md")
}

// NovelExists reports whether a session has a saved novelization.
func (s *Storage) NovelExists(name string) bool {
	_, err := os.Stat(s.sessionNovelPath(name))
	return err == nil
}

// LoadNovel reads a session's saved novelization. It returns os.ErrNotExist
// (wrapped) when none has been generated yet.
func (s *Storage) LoadNovel(name string) (string, error) {
	data, err := os.ReadFile(s.sessionNovelPath(name))
	if err != nil {
		return "", err
	}
	return string(data), nil
}

// SaveNovel writes a session's novelization, replacing any previous one. The
// write is atomic (temp file + rename) so a crash mid-write can't truncate an
// existing novel.
func (s *Storage) SaveNovel(name, md string) error {
	dir := filepath.Join(s.basePath, SessionsDir)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}
	path := s.sessionNovelPath(name)
	tmp, err := os.CreateTemp(dir, name+".novel.*.tmp")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	if _, err := tmp.WriteString(md); err != nil {
		_ = tmp.Close()
		_ = os.Remove(tmpName)
		return err
	}
	if err := tmp.Close(); err != nil {
		_ = os.Remove(tmpName)
		return err
	}
	if err := os.Rename(tmpName, path); err != nil {
		_ = os.Remove(tmpName)
		return fmt.Errorf("failed to write session novel: %w", err)
	}
	return nil
}

// DeleteNovel removes a session's novelization if present (no error if absent).
func (s *Storage) DeleteNovel(name string) error {
	if err := os.Remove(s.sessionNovelPath(name)); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("failed to delete session novel: %w", err)
	}
	return nil
}
