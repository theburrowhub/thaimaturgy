package storage

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/theburrowhub/thaimaturgy/internal/domain"
)

// UsersDir holds the registered-user accounts (#151): one JSON file per user,
// keyed by a non-reusable id, mirroring the campaign roster's layout.
const UsersDir = "users"

func (s *Storage) usersDir() string { return filepath.Join(s.basePath, UsersDir) }

func (s *Storage) userPath(id string) string {
	return filepath.Join(s.usersDir(), id+".json")
}

// User-store errors, surfaced to callers (and mapped to HTTP codes by the API).
var (
	ErrUsernameTaken = errors.New("username already taken")
	ErrUserNotFound  = errors.New("user not found")
)

// newUserID mints a NON-REUSABLE id: a username slug plus a random token, so
// deleting a user and recreating one with the same name yields a different id
// (a stale reference can't silently target the new account) — same rationale as
// roster ids.
func newUserID(username string) (string, error) {
	var buf [6]byte
	if _, err := rand.Read(buf[:]); err != nil {
		return "", fmt.Errorf("failed to generate user id: %w", err)
	}
	return slugifyName(username) + "-" + hex.EncodeToString(buf[:]), nil
}

// CreateUser mints a new user with a unique (case-insensitive) username and an
// optional password. Serialized against other user-store operations.
func (s *Storage) CreateUser(username string, role domain.UserRole, password string) (*domain.User, error) {
	s.usersMu.Lock()
	defer s.usersMu.Unlock()

	uname := strings.TrimSpace(username)
	if uname == "" {
		return nil, errors.New("username is required")
	}
	if !domain.ValidUserRole(role) {
		return nil, fmt.Errorf("invalid role %q", role)
	}
	existing, _ := s.listUsersLocked()
	for _, u := range existing {
		if domain.NormalizeUsername(u.Username) == domain.NormalizeUsername(uname) {
			return nil, ErrUsernameTaken
		}
	}
	id, err := newUserID(uname)
	if err != nil {
		return nil, err
	}
	u := &domain.User{ID: id, Username: uname, Role: role}
	if password != "" {
		if err := u.SetPassword(password); err != nil {
			return nil, err
		}
	}
	if err := s.saveUserLocked(u); err != nil {
		return nil, err
	}
	return u, nil
}

// SaveUser persists an existing user (updating in place by id). Use CreateUser
// for a new account so username uniqueness is enforced.
func (s *Storage) SaveUser(u *domain.User) error {
	s.usersMu.Lock()
	defer s.usersMu.Unlock()
	return s.saveUserLocked(u)
}

func (s *Storage) saveUserLocked(u *domain.User) error {
	if !validRosterID(u.ID) {
		return fmt.Errorf("invalid user id %q", u.ID)
	}
	if err := os.MkdirAll(s.usersDir(), 0o755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(u, "", "  ")
	if err != nil {
		return err
	}
	return atomicWriteFile(s.userPath(u.ID), data, 0o600) // 0600: contains a password hash
}

// LoadUser reads a user by id.
func (s *Storage) LoadUser(id string) (*domain.User, error) {
	s.usersMu.Lock()
	defer s.usersMu.Unlock()
	return s.loadUserLocked(id)
}

func (s *Storage) loadUserLocked(id string) (*domain.User, error) {
	if !validRosterID(id) {
		return nil, fmt.Errorf("invalid user id %q", id)
	}
	data, err := os.ReadFile(s.userPath(id))
	if err != nil {
		if os.IsNotExist(err) {
			return nil, ErrUserNotFound
		}
		return nil, err
	}
	var u domain.User
	if err := json.Unmarshal(data, &u); err != nil {
		return nil, err
	}
	u.ID = id
	return &u, nil
}

// UserByUsername resolves a user by username (case-insensitive), or ErrUserNotFound.
func (s *Storage) UserByUsername(username string) (*domain.User, error) {
	s.usersMu.Lock()
	defer s.usersMu.Unlock()
	users, err := s.listUsersLocked()
	if err != nil {
		return nil, err
	}
	want := domain.NormalizeUsername(username)
	for _, u := range users {
		if domain.NormalizeUsername(u.Username) == want {
			return u, nil
		}
	}
	return nil, ErrUserNotFound
}

// ListUsers returns every user, sorted by username.
func (s *Storage) ListUsers() ([]*domain.User, error) {
	s.usersMu.Lock()
	defer s.usersMu.Unlock()
	return s.listUsersLocked()
}

func (s *Storage) listUsersLocked() ([]*domain.User, error) {
	entries, err := os.ReadDir(s.usersDir())
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil // no users yet
		}
		return nil, err
	}
	var out []*domain.User
	var failed []string
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".json") {
			continue
		}
		id := strings.TrimSuffix(e.Name(), ".json")
		u, err := s.loadUserLocked(id)
		if err != nil {
			failed = append(failed, e.Name())
			continue
		}
		out = append(out, u)
	}
	sort.Slice(out, func(i, j int) bool {
		return domain.NormalizeUsername(out[i].Username) < domain.NormalizeUsername(out[j].Username)
	})
	if len(failed) > 0 {
		return out, fmt.Errorf("%d user file(s) could not be read: %s", len(failed), strings.Join(failed, ", "))
	}
	return out, nil
}

// DeleteUser removes a user by id (no error if it's already gone).
func (s *Storage) DeleteUser(id string) error {
	s.usersMu.Lock()
	defer s.usersMu.Unlock()
	if !validRosterID(id) {
		return fmt.Errorf("invalid user id %q", id)
	}
	if err := os.Remove(s.userPath(id)); err != nil && !os.IsNotExist(err) {
		return err
	}
	return nil
}

// UserCount reports how many users are registered (used to bootstrap the first
// admin and to decide whether the single-token fallback still applies).
func (s *Storage) UserCount() (int, error) {
	users, err := s.ListUsers()
	return len(users), err
}
