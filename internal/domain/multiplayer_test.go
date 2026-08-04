package domain

import "testing"

func partyState() *SessionState {
	st := NewSessionState("mp", nil)
	st.Characters = []*Character{
		NewCharacter("Alden", "Human", "Fighter"),
		NewCharacter("Naivara", "Elf", "Wizard"),
	}
	return st
}

func TestClaimCharacter(t *testing.T) {
	st := partyState()

	if _, err := st.ClaimCharacter("p1", "Ana", "Alden"); err != nil {
		t.Fatalf("claim Alden: %v", err)
	}
	if got := st.PlayerCharacterName("p1"); got != "Alden" {
		t.Errorf("p1 controls %q, want Alden", got)
	}
	// A different player can't take the same character.
	if _, err := st.ClaimCharacter("p2", "Luis", "alden"); err == nil {
		t.Error("expected conflict claiming an owned character")
	}
	// Unknown character is rejected.
	if _, err := st.ClaimCharacter("p2", "Luis", "Gandalf"); err == nil {
		t.Error("expected error for unknown character")
	}
	// Second player takes a free character.
	if _, err := st.ClaimCharacter("p2", "Luis", "Naivara"); err != nil {
		t.Fatalf("claim Naivara: %v", err)
	}
	if st.PlayerCount() != 2 {
		t.Errorf("player count = %d, want 2", st.PlayerCount())
	}
}

func TestSubmitActionAndRound(t *testing.T) {
	st := partyState()
	if _, err := st.SubmitAction("p1", "I attack"); err == nil {
		t.Error("submitting without a character should fail")
	}
	_, _ = st.ClaimCharacter("p1", "Ana", "Alden")
	_, _ = st.ClaimCharacter("p2", "Luis", "Naivara")

	if _, err := st.SubmitAction("p1", "I kick the door"); err != nil {
		t.Fatalf("submit: %v", err)
	}
	// Resubmitting replaces, not appends.
	if _, err := st.SubmitAction("p1", "I force the door open"); err != nil {
		t.Fatalf("resubmit: %v", err)
	}
	if n := len(st.RoundActions()); n != 1 {
		t.Fatalf("round actions = %d, want 1 (replace)", n)
	}
	if st.RoundActions()[0].Text != "I force the door open" {
		t.Errorf("action not replaced: %q", st.RoundActions()[0].Text)
	}
	// p2 still pending.
	if pend := st.PendingPlayers(); len(pend) != 1 || pend[0] != "Luis" {
		t.Errorf("pending = %v, want [Luis]", pend)
	}
	_, _ = st.SubmitAction("p2", "I ready a spell")
	if len(st.PendingPlayers()) != 0 {
		t.Errorf("no players should be pending, got %v", st.PendingPlayers())
	}
	if len(st.RoundActions()) != 2 {
		t.Errorf("round actions = %d, want 2", len(st.RoundActions()))
	}

	st.ResetRound()
	if len(st.RoundActions()) != 0 {
		t.Error("round should be empty after reset")
	}

	// Switching to a different character drops the stale pending action, so it
	// isn't attributed to a character the player no longer controls.
	_, _ = st.SubmitAction("p1", "I sneak forward")
	if _, err := st.ClaimCharacter("p1", "Ana", "Naivara"); err == nil {
		t.Error("Naivara is taken by p2; claim should fail")
	}
	// Free a character, then switch p1 onto it.
	st.ReleaseCharacter("p2")
	if _, err := st.ClaimCharacter("p1", "Ana", "Naivara"); err != nil {
		t.Fatalf("switch p1 to Naivara: %v", err)
	}
	for _, a := range st.RoundActions() {
		if a.PlayerID == "p1" {
			t.Errorf("p1's stale action should have been dropped on character switch, got %q for %s", a.Text, a.CharacterName)
		}
	}

	// RemoveResolvedActions drops only the snapshotted actions, keeping ones
	// submitted afterwards (e.g. during DM resolution).
	st.ResetRound()
	_, _ = st.ClaimCharacter("p2", "Luis", "Alden")
	a1, _ := st.SubmitAction("p1", "cast light")
	snapshot := []RoundAction{a1}
	_, _ = st.SubmitAction("p2", "draw sword") // submitted "while resolving"
	st.RemoveResolvedActions(snapshot)
	rem := st.RoundActions()
	if len(rem) != 1 || rem[0].PlayerID != "p2" {
		t.Errorf("only p2's later action should remain, got %+v", rem)
	}
	// Clean up for the release check below: single player (p1) with an empty round.
	st.ResetRound()
	st.ReleaseCharacter("p2")

	// Releasing a character drops the player and any pending action.
	_, _ = st.SubmitAction("p1", "again")
	st.ReleaseCharacter("p1")
	if st.PlayerCharacterName("p1") != "" {
		t.Error("released player should control no character")
	}
	if len(st.RoundActions()) != 0 {
		t.Error("released player's action should be dropped")
	}
}
