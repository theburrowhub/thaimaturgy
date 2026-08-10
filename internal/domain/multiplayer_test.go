package domain

import (
	"strings"
	"testing"
)

func partyState() *SessionState {
	st := NewSessionState("mp", nil)
	st.Characters = []*Character{
		NewCharacter("Alden", "Human", "Fighter"),
		NewCharacter("Naivara", "Elf", "Wizard"),
	}
	return st
}

func TestAssignByUsernameAndResolve(t *testing.T) {
	st := partyState() // Alden, Naivara

	// Reserve Naivara for @luis (who hasn't picked yet).
	if _, err := st.AssignByUsername("@Luis", "Naivara"); err != nil {
		t.Fatalf("assign: %v", err)
	}
	if got := st.PendingByCharacter()["Naivara"]; got != "luis" {
		t.Errorf("pending for Naivara = %q, want luis", got)
	}
	// Can't reserve the same character for someone else.
	if _, err := st.AssignByUsername("@ana", "naivara"); err == nil {
		t.Error("reserving an already-reserved character should fail")
	}
	// Unknown character rejected.
	if _, err := st.AssignByUsername("@ana", "Gandalf"); err == nil {
		t.Error("unknown character should fail")
	}

	// A message from a different user does not bind Luis's reservation.
	if _, bound := st.ResolvePending("999", "someoneelse", "Someone"); bound {
		t.Error("unrelated user should not bind the reservation")
	}
	// Luis appears → bound to Naivara, pending cleared.
	name, bound := st.ResolvePending("42", "luis", "Luis")
	if !bound || name != "Naivara" {
		t.Fatalf("resolve for luis = (%q,%v), want (Naivara,true)", name, bound)
	}
	if st.PlayerCharacterName("42") != "Naivara" {
		t.Error("Luis should control Naivara after binding")
	}
	if len(st.PendingByCharacter()) != 0 {
		t.Error("pending assignment should be cleared after binding")
	}
	// Second appearance is a no-op (already controls one).
	if _, bound := st.ResolvePending("42", "luis", "Luis"); bound {
		t.Error("resolve should be a no-op once the player controls a character")
	}
}

func TestGameLifecycle(t *testing.T) {
	st := partyState()
	if st.GameStarted() {
		t.Fatal("a new game should not be started")
	}
	if !st.StartGame() {
		t.Error("StartGame should report it started the game")
	}
	if !st.GameStarted() {
		t.Error("game should be started now")
	}
	if st.StartGame() {
		t.Error("StartGame should be idempotent (false the second time)")
	}
}

func TestMarkStartedIfInProgress(t *testing.T) {
	// Fresh session: no evidence of play → stays not-started.
	fresh := partyState()
	fresh.MarkStartedIfInProgress()
	if fresh.GameStarted() {
		t.Error("a fresh session should not be marked started")
	}

	// A session where the DM has narrated → treated as started.
	played := partyState()
	played.Conversation.AddAssistantMessage("You stand at the gate.")
	played.MarkStartedIfInProgress()
	if !played.GameStarted() {
		t.Error("a session with prior narration should be marked started")
	}
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
	if _, err := st.SubmitAction("p1", "", "I attack"); err == nil {
		t.Error("submitting without a character should fail")
	}
	_, _ = st.ClaimCharacter("p1", "Ana", "Alden")
	_, _ = st.ClaimCharacter("p2", "Luis", "Naivara")

	if _, err := st.SubmitAction("p1", "", "I kick the door"); err != nil {
		t.Fatalf("submit: %v", err)
	}
	// Resubmitting replaces, not appends.
	if _, err := st.SubmitAction("p1", "", "I force the door open"); err != nil {
		t.Fatalf("resubmit: %v", err)
	}
	if n := len(st.RoundActions()); n != 1 {
		t.Fatalf("round actions = %d, want 1 (replace)", n)
	}
	if st.RoundActions()[0].Text != "I force the door open" {
		t.Errorf("action not replaced: %q", st.RoundActions()[0].Text)
	}
	// p2 (Naivara) still pending — PendingPlayers lists the character with player.
	pend := st.PendingPlayers()
	if len(pend) != 1 || !strings.Contains(pend[0], "Naivara") {
		t.Errorf("pending = %v, want [Naivara (Luis)]", pend)
	}
	_, _ = st.SubmitAction("p2", "", "I ready a spell")
	if len(st.PendingPlayers()) != 0 {
		t.Errorf("no characters should be pending, got %v", st.PendingPlayers())
	}
	if len(st.RoundActions()) != 2 {
		t.Errorf("round actions = %d, want 2", len(st.RoundActions()))
	}

	st.ResetRound()

	// RemoveResolvedActions drops only the snapshotted actions, keeping ones
	// submitted afterwards (e.g. during DM resolution).
	a1, _ := st.SubmitAction("p1", "", "cast light")
	snapshot := []RoundAction{a1}
	_, _ = st.SubmitAction("p2", "", "draw sword") // submitted "while resolving"
	st.RemoveResolvedActions(snapshot)
	rem := st.RoundActions()
	if len(rem) != 1 || rem[0].PlayerID != "p2" {
		t.Errorf("only p2's later action should remain, got %+v", rem)
	}
	st.ResetRound()
	st.ReleaseCharacter("p2")

	// Releasing a player drops them and any pending action.
	_, _ = st.SubmitAction("p1", "", "again")
	st.ReleaseCharacter("p1")
	if st.PlayerCharacterName("p1") != "" {
		t.Error("released player should control no character")
	}
	if len(st.RoundActions()) != 0 {
		t.Error("released player's action should be dropped")
	}
}

// TestMultiCharacterPerPlayer covers #29: one player controlling several PCs,
// choosing the active one, targeting a specific PC per action, and acting for
// each in a round.
func TestMultiCharacterPerPlayer(t *testing.T) {
	st := NewSessionState("mp", nil)
	st.Characters = []*Character{
		NewCharacter("Alden", "Human", "Fighter"),
		NewCharacter("Naivara", "Elf", "Wizard"),
		NewCharacter("Thorin", "Dwarf", "Cleric"),
	}

	// p1 claims two characters; the last claimed is active.
	if _, err := st.ClaimCharacter("p1", "Ana", "Alden"); err != nil {
		t.Fatalf("claim Alden: %v", err)
	}
	if _, err := st.ClaimCharacter("p1", "Ana", "Thorin"); err != nil {
		t.Fatalf("claim Thorin: %v", err)
	}
	names := st.PlayerCharacterNames("p1")
	if len(names) != 2 {
		t.Fatalf("p1 should control 2 characters, got %v", names)
	}
	if st.PlayerCharacterName("p1") != "Thorin" {
		t.Errorf("active should be the last claimed (Thorin), got %q", st.PlayerCharacterName("p1"))
	}

	// Choose the active character explicitly.
	if _, err := st.SetActiveCharacter("p1", "alden"); err != nil {
		t.Fatalf("set active: %v", err)
	}
	if st.PlayerCharacterName("p1") != "Alden" {
		t.Errorf("active = %q, want Alden", st.PlayerCharacterName("p1"))
	}
	// Can't activate a character you don't control.
	if _, err := st.SetActiveCharacter("p1", "Naivara"); err == nil {
		t.Error("activating an uncontrolled character should fail")
	}

	// Act for each character: one targeted, one via the active default.
	if a, err := st.SubmitAction("p1", "Thorin", "cast bless"); err != nil || a.CharacterName != "Thorin" {
		t.Fatalf("submit for Thorin = (%+v, %v)", a, err)
	}
	if a, err := st.SubmitAction("p1", "", "attack"); err != nil || a.CharacterName != "Alden" {
		t.Fatalf("submit for active Alden = (%+v, %v)", a, err)
	}
	if n := len(st.RoundActions()); n != 2 {
		t.Errorf("a player with 2 PCs should have 2 actions, got %d", n)
	}
	// Targeting a character the player doesn't control is rejected.
	if _, err := st.SubmitAction("p1", "Naivara", "flee"); err == nil {
		t.Error("acting for an uncontrolled character should fail")
	}

	// Another player can't claim one of p1's characters.
	if _, err := st.ClaimCharacter("p2", "Bob", "Alden"); err == nil {
		t.Error("claiming a character controlled by another player should fail")
	}
}
