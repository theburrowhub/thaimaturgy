package domain

import "testing"

func TestUserPassword(t *testing.T) {
	u := &User{Username: "aria", Role: RolePlayer}
	if u.CheckPassword("anything") {
		t.Error("a user with no password set must never authenticate")
	}
	if err := u.SetPassword("s3cret!"); err != nil {
		t.Fatalf("SetPassword: %v", err)
	}
	if u.PasswordHash == "" || u.PasswordHash == "s3cret!" {
		t.Error("password must be stored as a non-empty hash, never plaintext")
	}
	if !u.CheckPassword("s3cret!") {
		t.Error("correct password should verify")
	}
	if u.CheckPassword("wrong") {
		t.Error("wrong password must not verify")
	}
}

func TestUserCharacterAssignment(t *testing.T) {
	u := &User{Username: "dm", Role: RoleAdmin}
	if !u.IsAdmin() {
		t.Error("admin role should report IsAdmin")
	}
	if u.AssignCharacter("") {
		t.Error("assigning an empty id should be a no-op")
	}
	if !u.AssignCharacter("aria-abc123") {
		t.Error("first assignment should report a change")
	}
	if u.AssignCharacter("aria-abc123") {
		t.Error("re-assigning the same id should be a no-op")
	}
	if !u.HasCharacter("aria-abc123") {
		t.Error("HasCharacter should find the assigned id")
	}
	u.AssignCharacter("borin-def456")
	if !u.UnassignCharacter("aria-abc123") || u.HasCharacter("aria-abc123") {
		t.Error("unassign should remove the id")
	}
	if u.UnassignCharacter("nope") {
		t.Error("unassigning an absent id should report no change")
	}
	if len(u.CharacterIDs) != 1 || u.CharacterIDs[0] != "borin-def456" {
		t.Errorf("remaining assignments = %v; want [borin-def456]", u.CharacterIDs)
	}
}

func TestUserSanitizedStripsHash(t *testing.T) {
	u := &User{Username: "aria", Role: RolePlayer, CharacterIDs: []string{"x"}}
	_ = u.SetPassword("pw")
	s := u.Sanitized()
	if s.PasswordHash != "" {
		t.Error("Sanitized must strip the password hash")
	}
	if u.PasswordHash == "" {
		t.Error("Sanitized must not mutate the original")
	}
	// The clone's slice must be independent of the original.
	s.CharacterIDs[0] = "mutated"
	if u.CharacterIDs[0] != "x" {
		t.Error("Sanitized must deep-copy CharacterIDs")
	}
}
