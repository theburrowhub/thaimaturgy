package storage

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/theburrowhub/thaimaturgy/internal/domain"
)

// CharactersDir holds the persistent campaign roster: player characters that
// outlive any single session, so they can be selected into new adventures and
// carry their progression forward (issue #33).
const CharactersDir = "characters"

func (s *Storage) charactersDir() string { return filepath.Join(s.basePath, CharactersDir) }

func (s *Storage) characterPath(id string) string {
	return filepath.Join(s.charactersDir(), id+".json")
}

// slugifyName derives a readable, filesystem-safe prefix from a character name.
// Falls back to "character" when nothing survives.
func slugifyName(name string) string {
	var b strings.Builder
	lastDash := false
	for _, r := range strings.ToLower(strings.TrimSpace(name)) {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9':
			b.WriteRune(r)
			lastDash = false
		default:
			if !lastDash {
				b.WriteByte('-')
				lastDash = true
			}
		}
	}
	slug := strings.Trim(b.String(), "-")
	if slug == "" {
		return "character"
	}
	return slug
}

// newRosterID mints a NON-REUSABLE id: a readable name slug plus a random token.
// Because the token is random, deleting a character and creating another with the
// same name yields a different id, so a stale session link (which stores the old
// id) can never silently target the new entry (Heimdallm review).
func newRosterID(name string) (string, error) {
	var buf [6]byte
	if _, err := rand.Read(buf[:]); err != nil {
		return "", fmt.Errorf("failed to generate character id: %w", err)
	}
	return slugifyName(name) + "-" + hex.EncodeToString(buf[:]), nil
}

// validRosterID guards against path traversal in a caller-supplied id.
func validRosterID(id string) bool {
	return id != "" && !strings.ContainsAny(id, `/\`) && !strings.Contains(id, "..")
}

// SaveCharacter persists a character to the roster and returns its id. A new
// character (empty ID) is assigned a fresh non-reusable id; an existing ID updates
// that entry in place (how a session writes progression back). The id is written
// back into the passed character. Serialized against other roster operations.
func (s *Storage) SaveCharacter(c *domain.Character) (string, error) {
	s.rosterMu.Lock()
	defer s.rosterMu.Unlock()
	return s.saveCharacterLocked(c)
}

// saveCharacterLocked is the lock-free core of SaveCharacter (caller holds
// rosterMu), so SyncPartyToRoster can save without re-locking.
func (s *Storage) saveCharacterLocked(c *domain.Character) (string, error) {
	if c == nil {
		return "", fmt.Errorf("nil character")
	}
	if strings.TrimSpace(c.Name) == "" {
		return "", fmt.Errorf("character name is required")
	}
	if err := os.MkdirAll(s.charactersDir(), 0755); err != nil {
		return "", fmt.Errorf("failed to create characters directory: %w", err)
	}
	if c.ID == "" {
		id, err := newRosterID(c.Name)
		if err != nil {
			return "", err
		}
		c.ID = id
	} else if !validRosterID(c.ID) {
		return "", fmt.Errorf("invalid character id: %q", c.ID)
	}
	data, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return "", fmt.Errorf("failed to marshal character: %w", err)
	}
	// Atomic write: a partial/failed write (disk full, interruption) must not
	// destroy the existing character file.
	if err := atomicWriteFile(s.characterPath(c.ID), data, 0644); err != nil {
		return "", fmt.Errorf("failed to write character file: %w", err)
	}
	return c.ID, nil
}

// LoadCharacter reads a roster character by id (ensuring its ID field is set).
func (s *Storage) LoadCharacter(id string) (*domain.Character, error) {
	s.rosterMu.Lock()
	defer s.rosterMu.Unlock()
	return s.loadCharacterLocked(id)
}

func (s *Storage) loadCharacterLocked(id string) (*domain.Character, error) {
	if !validRosterID(id) {
		return nil, fmt.Errorf("invalid character id: %q", id)
	}
	data, err := os.ReadFile(s.characterPath(id))
	if err != nil {
		return nil, fmt.Errorf("failed to read character file: %w", err)
	}
	var c domain.Character
	if err := json.Unmarshal(data, &c); err != nil {
		return nil, fmt.Errorf("failed to parse character file: %w", err)
	}
	c.ID = id
	return &c, nil
}

// ListCharacters returns every roster character, sorted by name. It decodes as
// many entries as it can and, if any file is unreadable/corrupt, returns the
// successfully decoded characters ALONGSIDE a non-nil error naming the failures,
// so a caller can surface an incomplete roster rather than silently dropping
// entries (Heimdallm review).
func (s *Storage) ListCharacters() ([]*domain.Character, error) {
	s.rosterMu.Lock()
	defer s.rosterMu.Unlock()
	entries, err := os.ReadDir(s.charactersDir())
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to read characters directory: %w", err)
	}
	var out []*domain.Character
	var failed []string
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".json") {
			continue
		}
		id := strings.TrimSuffix(entry.Name(), ".json")
		c, err := s.loadCharacterLocked(id)
		if err != nil {
			failed = append(failed, id)
			continue
		}
		out = append(out, c)
	}
	sort.Slice(out, func(i, j int) bool {
		return strings.ToLower(out[i].Name) < strings.ToLower(out[j].Name)
	})
	if len(failed) > 0 {
		return out, fmt.Errorf("%d roster file(s) could not be read: %s", len(failed), strings.Join(failed, ", "))
	}
	return out, nil
}

// DeleteCharacter removes a roster character by id.
func (s *Storage) DeleteCharacter(id string) error {
	s.rosterMu.Lock()
	defer s.rosterMu.Unlock()
	if !validRosterID(id) {
		return fmt.Errorf("invalid character id: %q", id)
	}
	if err := os.Remove(s.characterPath(id)); err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("character not found: %s", id)
		}
		return fmt.Errorf("failed to delete character file: %w", err)
	}
	return nil
}

// SyncPartyToRoster writes the progression of roster-linked party members (those
// with a non-empty ID that still exists in the roster) back to the roster. The
// existence check and write happen under the same lock, so a concurrent delete
// can't be raced into resurrecting an entry. Ad-hoc members (no ID) and members
// whose roster entry was deleted are left untouched, so a session never silently
// creates or resurrects roster entries. Returns the number of entries updated.
func (s *Storage) SyncPartyToRoster(party []*domain.Character) (int, error) {
	s.rosterMu.Lock()
	defer s.rosterMu.Unlock()
	updated := 0
	for _, c := range party {
		if c == nil || c.ID == "" || !validRosterID(c.ID) {
			continue
		}
		// Only skip a member whose roster entry genuinely doesn't exist (deleted).
		// A permission/I/O error must be propagated, not silently treated as
		// "deleted" — otherwise progression is lost with no signal.
		if _, err := os.Stat(s.characterPath(c.ID)); err != nil {
			if os.IsNotExist(err) {
				continue
			}
			return updated, fmt.Errorf("failed to check roster entry %s: %w", c.ID, err)
		}
		if _, err := s.saveCharacterLocked(c); err != nil {
			return updated, err
		}
		updated++
	}
	return updated, nil
}
