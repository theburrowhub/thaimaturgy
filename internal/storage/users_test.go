package storage

import (
	"errors"
	"testing"

	"github.com/theburrowhub/thaimaturgy/internal/domain"
)

func TestUserStoreCRUD(t *testing.T) {
	s, err := NewWithPath(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}

	if n, err := s.UserCount(); err != nil || n != 0 {
		t.Fatalf("fresh store: count=%d err=%v; want 0", n, err)
	}

	admin, err := s.CreateUser("Admin", domain.RoleAdmin, "pw1")
	if err != nil {
		t.Fatalf("CreateUser admin: %v", err)
	}
	if admin.ID == "" || admin.Role != domain.RoleAdmin || admin.PasswordHash == "" {
		t.Fatalf("unexpected admin: %+v", admin)
	}

	// Case-insensitive username uniqueness.
	if _, err := s.CreateUser("admin", domain.RolePlayer, "x"); !errors.Is(err, ErrUsernameTaken) {
		t.Errorf("duplicate username (different case) should be rejected, got %v", err)
	}

	player, err := s.CreateUser("Aria", domain.RolePlayer, "")
	if err != nil {
		t.Fatalf("CreateUser player: %v", err)
	}

	// Lookup by username (case-insensitive) and by id.
	if got, err := s.UserByUsername("ARIA"); err != nil || got.ID != player.ID {
		t.Errorf("UserByUsername = %+v, %v; want %s", got, err, player.ID)
	}
	if got, err := s.LoadUser(admin.ID); err != nil || got.Username != "Admin" {
		t.Errorf("LoadUser = %+v, %v", got, err)
	}
	if _, err := s.LoadUser("does-not-exist"); !errors.Is(err, ErrUserNotFound) {
		t.Errorf("missing user should be ErrUserNotFound, got %v", err)
	}

	// List is sorted by username.
	users, err := s.ListUsers()
	if err != nil || len(users) != 2 {
		t.Fatalf("ListUsers = %d, %v; want 2", len(users), err)
	}
	if users[0].Username != "Admin" || users[1].Username != "Aria" {
		t.Errorf("users not sorted by username: %q, %q", users[0].Username, users[1].Username)
	}

	// Persisted password verifies after a reload (survives round-trip).
	reloaded, _ := s.LoadUser(admin.ID)
	if !reloaded.CheckPassword("pw1") {
		t.Error("password did not survive persistence round-trip")
	}

	// Assign a character and persist.
	player.AssignCharacter("aria-abc123")
	if err := s.SaveUser(player); err != nil {
		t.Fatalf("SaveUser: %v", err)
	}
	if got, _ := s.LoadUser(player.ID); !got.HasCharacter("aria-abc123") {
		t.Error("character assignment did not persist")
	}

	// Delete is idempotent.
	if err := s.DeleteUser(player.ID); err != nil {
		t.Fatalf("DeleteUser: %v", err)
	}
	if err := s.DeleteUser(player.ID); err != nil {
		t.Errorf("deleting an absent user should be a no-op, got %v", err)
	}
	if n, _ := s.UserCount(); n != 1 {
		t.Errorf("count after delete = %d; want 1", n)
	}
}

func TestCreateUserValidation(t *testing.T) {
	s, _ := NewWithPath(t.TempDir())
	if _, err := s.CreateUser("  ", domain.RoleAdmin, "pw"); err == nil {
		t.Error("blank username should be rejected")
	}
	if _, err := s.CreateUser("bob", domain.UserRole("wizard"), "pw"); err == nil {
		t.Error("invalid role should be rejected")
	}
}
