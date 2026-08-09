package storage

import (
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

// slugifyName derives a filesystem-safe id from a character name (lower-case,
// alphanumerics and hyphens). Falls back to "character" when nothing survives.
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

// validRosterID guards against path traversal in a caller-supplied id.
func validRosterID(id string) bool {
	return id != "" && !strings.ContainsAny(id, `/\`) && !strings.Contains(id, "..")
}

// SaveCharacter persists a character to the roster and returns its id. If the
// character has no ID yet, one is derived from its name and made unique against
// existing entries; the ID is written back into the passed character so the
// caller can keep the link. An existing ID updates that entry in place (this is
// how a session writes progression back).
func (s *Storage) SaveCharacter(c *domain.Character) (string, error) {
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
		c.ID = s.uniqueRosterID(slugifyName(c.Name))
	} else if !validRosterID(c.ID) {
		return "", fmt.Errorf("invalid character id: %q", c.ID)
	}
	data, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return "", fmt.Errorf("failed to marshal character: %w", err)
	}
	if err := os.WriteFile(s.characterPath(c.ID), data, 0644); err != nil {
		return "", fmt.Errorf("failed to write character file: %w", err)
	}
	return c.ID, nil
}

// uniqueRosterID returns base, or base-2, base-3… so a second "Alice" doesn't
// overwrite the first.
func (s *Storage) uniqueRosterID(base string) string {
	id := base
	for i := 2; s.characterExists(id); i++ {
		id = fmt.Sprintf("%s-%d", base, i)
	}
	return id
}

func (s *Storage) characterExists(id string) bool {
	_, err := os.Stat(s.characterPath(id))
	return err == nil
}

// LoadCharacter reads a roster character by id (ensuring its ID field is set).
func (s *Storage) LoadCharacter(id string) (*domain.Character, error) {
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

// ListCharacters returns every roster character, sorted by name.
func (s *Storage) ListCharacters() ([]*domain.Character, error) {
	entries, err := os.ReadDir(s.charactersDir())
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to read characters directory: %w", err)
	}
	var out []*domain.Character
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".json") {
			continue
		}
		c, err := s.LoadCharacter(strings.TrimSuffix(entry.Name(), ".json"))
		if err != nil {
			continue // skip unreadable entries rather than failing the whole list
		}
		out = append(out, c)
	}
	sort.Slice(out, func(i, j int) bool {
		return strings.ToLower(out[i].Name) < strings.ToLower(out[j].Name)
	})
	return out, nil
}

// DeleteCharacter removes a roster character by id.
func (s *Storage) DeleteCharacter(id string) error {
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
// with a non-empty ID that still exists in the roster) back to the roster. Ad-hoc
// members (no ID) and members whose roster entry was deleted are left untouched,
// so a session never silently creates or resurrects roster entries. Returns the
// number of entries updated.
func (s *Storage) SyncPartyToRoster(party []*domain.Character) (int, error) {
	updated := 0
	for _, c := range party {
		if c == nil || c.ID == "" || !s.characterExists(c.ID) {
			continue
		}
		if _, err := s.SaveCharacter(c); err != nil {
			return updated, err
		}
		updated++
	}
	return updated, nil
}
