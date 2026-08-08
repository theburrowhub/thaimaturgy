package engine

import (
	"strings"
	"testing"

	"github.com/theburrowhub/thaimaturgy/internal/domain"
)

// TestDMContextIncludesCurrentSheets verifies the DM's system prompt always
// carries each party member's CURRENT sheet (HP/conditions) and the rule that
// narration must not contradict it — the fix for state-incongruence bugs (#32).
func TestDMContextIncludesCurrentSheets(t *testing.T) {
	session := createTestSession()
	session.State.Mode = domain.ModeVirtualDM
	session.State.SetParty([]*domain.Character{
		{Name: "Borin", Race: "Dwarf", Class: "Fighter", Level: 3, MaxHP: 30, CurrentHP: 2, AC: 16,
			Conditions: []domain.Condition{domain.ConditionProne}},
	})

	prompt := NewOracle(session, nil).buildSystemPrompt()

	if !strings.Contains(prompt, "Borin") || !strings.Contains(prompt, "HP: 2/30") {
		t.Fatalf("prompt missing current sheet HP for Borin:\n%s", prompt)
	}
	if !strings.Contains(prompt, "Prone") {
		t.Errorf("prompt missing current condition (Prone)")
	}
	if !strings.Contains(strings.ToLower(prompt), "authoritative") {
		t.Errorf("prompt missing the authoritative-sheet instruction")
	}
}
