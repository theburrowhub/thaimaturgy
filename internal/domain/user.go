package domain

import (
	"slices"
	"strings"

	"golang.org/x/crypto/bcrypt"
)

// UserRole is a registered user's authorization level for remote access (#151).
type UserRole string

const (
	// RoleAdmin has full control: manage users and the whole roster.
	RoleAdmin UserRole = "admin"
	// RolePlayer may play and manage only their own assigned characters.
	RolePlayer UserRole = "player"
)

// ValidUserRole reports whether r is a known role.
func ValidUserRole(r UserRole) bool { return r == RoleAdmin || r == RolePlayer }

// User is a registered account for remote access (GUI + web). Only registered
// users may use the system; each user may have one or more assigned characters
// from the campaign roster (#151). It never carries a plaintext password — only a
// bcrypt hash — so it is safe to persist and to return over the API when the hash
// is stripped by the transport layer.
type User struct {
	ID           string   `json:"id"`
	Username     string   `json:"username"`
	Role         UserRole `json:"role"`
	PasswordHash string   `json:"password_hash,omitempty"` // bcrypt; empty until a password is set
	TelegramID   string   `json:"telegram_id,omitempty"`   // linked Telegram user id (#152)
	CharacterIDs []string `json:"character_ids,omitempty"` // assigned roster character ids
}

// NormalizeUsername canonicalizes a username for case-insensitive uniqueness and
// lookup (trim + lower-case).
func NormalizeUsername(s string) string { return strings.ToLower(strings.TrimSpace(s)) }

// SetPassword stores a bcrypt hash of pw (never the plaintext).
func (u *User) SetPassword(pw string) error {
	h, err := bcrypt.GenerateFromPassword([]byte(pw), bcrypt.DefaultCost)
	if err != nil {
		return err
	}
	u.PasswordHash = string(h)
	return nil
}

// CheckPassword reports whether pw matches the stored hash. It is always false
// when no password has been set (a hash-less account can't be logged into).
func (u *User) CheckPassword(pw string) bool {
	if u.PasswordHash == "" {
		return false
	}
	return bcrypt.CompareHashAndPassword([]byte(u.PasswordHash), []byte(pw)) == nil
}

// IsAdmin reports whether the user has the admin role.
func (u *User) IsAdmin() bool { return u.Role == RoleAdmin }

// HasCharacter reports whether the roster character id is assigned to the user.
func (u *User) HasCharacter(id string) bool { return slices.Contains(u.CharacterIDs, id) }

// AssignCharacter links a roster character to the user (no-op if already linked
// or id is empty). Returns whether it changed.
func (u *User) AssignCharacter(id string) bool {
	if id == "" || u.HasCharacter(id) {
		return false
	}
	u.CharacterIDs = append(u.CharacterIDs, id)
	return true
}

// UnassignCharacter removes a roster character link, reporting whether it changed.
func (u *User) UnassignCharacter(id string) bool {
	for i, cid := range u.CharacterIDs {
		if cid == id {
			u.CharacterIDs = append(u.CharacterIDs[:i], u.CharacterIDs[i+1:]...)
			return true
		}
	}
	return false
}

// Sanitized returns a copy safe to expose over the API: the password hash is
// cleared so a secret never leaves the server.
func (u *User) Sanitized() *User {
	c := *u
	c.PasswordHash = ""
	c.CharacterIDs = slices.Clone(u.CharacterIDs)
	return &c
}
