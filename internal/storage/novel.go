package storage

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// The novelization of a session is stored as a sibling Markdown file next to the
// session JSON (mirroring the journal at "<name>.journal.md"), so the possibly
// large prose stays out of the session file and can be re-opened and edited.
//
// The session name becomes a filename, so it is validated and the resolved path
// is confirmed to stay inside the sessions directory — a name like
// "../../etc/passwd" must never let a caller read or clobber a file outside it.
func (s *Storage) sessionNovelPath(name string) (string, error) {
	if name == "" || strings.ContainsAny(name, `/\`) || strings.Contains(name, "..") || strings.ContainsRune(name, 0) {
		return "", fmt.Errorf("invalid session name: %q", name)
	}
	dir := filepath.Join(s.basePath, SessionsDir)
	p := filepath.Join(dir, name+".novel.md")
	// Defense in depth: the cleaned path must be exactly "<dir>/<name>.novel.md".
	if filepath.Dir(p) != dir {
		return "", fmt.Errorf("invalid session name: %q", name)
	}
	return p, nil
}

// NovelExists reports whether a session has a saved novelization. An invalid
// name (which can never have a valid novel path) reports false.
func (s *Storage) NovelExists(name string) bool {
	p, err := s.sessionNovelPath(name)
	if err != nil {
		return false
	}
	_, err = os.Stat(p)
	return err == nil
}

// LoadNovel reads a session's saved novelization. It returns os.ErrNotExist
// (wrapped) when none has been generated yet.
func (s *Storage) LoadNovel(name string) (string, error) {
	p, err := s.sessionNovelPath(name)
	if err != nil {
		return "", err
	}
	data, err := os.ReadFile(p)
	if err != nil {
		return "", err
	}
	return string(data), nil
}

// SaveNovel writes a session's novelization, replacing any previous one. The
// write is atomic (temp file + rename) so a crash mid-write can't truncate an
// existing novel.
func (s *Storage) SaveNovel(name, md string) error {
	path, err := s.sessionNovelPath(name)
	if err != nil {
		return err
	}
	dir := filepath.Join(s.basePath, SessionsDir)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(dir, "novel-*.tmp")
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
	p, err := s.sessionNovelPath(name)
	if err != nil {
		return err
	}
	if err := os.Remove(p); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("failed to delete session novel: %w", err)
	}
	return nil
}
