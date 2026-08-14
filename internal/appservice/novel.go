package appservice

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"os"
)

// novelVersion is a content hash used for optimistic-concurrency checks on the
// saved novel. An empty novel (none saved yet) has the empty version "".
func novelVersion(md string) string {
	if md == "" {
		return ""
	}
	sum := sha256.Sum256([]byte(md))
	return hex.EncodeToString(sum[:])
}

// NovelText returns a session's saved novelization, a version tag for
// optimistic concurrency, and whether one exists yet. A session with no novel
// returns ("", "", false, nil).
func (s *Service) NovelText(sessionName string) (md, version string, exists bool, err error) {
	s.novelMu.Lock()
	defer s.novelMu.Unlock()
	md, err = s.store.LoadNovel(sessionName)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return "", "", false, nil
		}
		return "", "", false, err
	}
	return md, novelVersion(md), true, nil
}

// SaveNovelText persists an edited novelization for a session, but only if the
// stored novel still matches baseVersion (optimistic concurrency); otherwise it
// returns ErrNovelConflict so the caller can reload and re-apply. It returns the
// new version tag on success. Pass the version from NovelText as baseVersion
// ("" when saving over no existing novel).
func (s *Service) SaveNovelText(sessionName, md, baseVersion string) (string, error) {
	s.novelMu.Lock()
	defer s.novelMu.Unlock()
	if err := s.checkNovelVersionLocked(sessionName, baseVersion); err != nil {
		return "", err
	}
	if err := s.store.SaveNovel(sessionName, md); err != nil {
		return "", err
	}
	return novelVersion(md), nil
}

// checkNovelVersionLocked compares the stored novel's version to want. Caller
// holds novelMu. A missing novel has version "".
func (s *Service) checkNovelVersionLocked(sessionName, want string) error {
	cur, err := s.store.LoadNovel(sessionName)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			cur = ""
		} else {
			return err
		}
	}
	if novelVersion(cur) != want {
		return ErrNovelConflict
	}
	return nil
}
