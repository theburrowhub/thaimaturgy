package engine

import (
	"strings"
	"testing"

	"github.com/theburrowhub/thaimaturgy/internal/domain"
)

func srdSession(npcs []domain.NPC) *domain.Session {
	adv := &domain.Adventure{
		SchemaVersion: domain.SchemaVersion, ID: "srd", Title: "SRD",
		Zones: []domain.Zone{{ID: "z", Name: "Z", Rooms: []domain.Room{{ID: "r", Name: "R"}}}},
		NPCs:  npcs,
	}
	return domain.NewSession(domain.NewSessionState("s", adv), adv, domain.DefaultConfig())
}

func TestLookupCreatureTool(t *testing.T) {
	tr := NewToolRouter(srdSession(nil))

	res := call(tr, "lookup_creature", map[string]any{"name": "goblin"})
	if res.Error != "" {
		t.Fatalf("lookup_creature goblin error: %s", res.Error)
	}
	if !strings.Contains(res.Content, "AC 15") || !strings.Contains(res.Content, "Scimitar") {
		t.Errorf("goblin stat block missing expected content:\n%s", res.Content)
	}

	res = call(tr, "lookup_creature", map[string]any{"name": "beholder"})
	if res.Error == "" {
		t.Errorf("expected error for a creature not in the subset, got:\n%s", res.Content)
	}
}

func TestGetNPCAutoFillsFromSRD(t *testing.T) {
	// An NPC named after a standard creature, with no authored stat block, gets an
	// SRD block auto-filled.
	tr := NewToolRouter(srdSession([]domain.NPC{{ID: "g1", Name: "Goblin"}}))
	res := call(tr, "get_npc", map[string]any{"npc_id": "g1"})
	if res.Error != "" {
		t.Fatalf("get_npc error: %s", res.Error)
	}
	if !strings.Contains(res.Content, "auto-filled") || !strings.Contains(res.Content, "AC 15") {
		t.Errorf("expected SRD auto-fill for a Goblin NPC:\n%s", res.Content)
	}

	// An authored stat block wins: no auto-fill.
	authored := []domain.NPC{{ID: "g2", Name: "Goblin", StatBlock: &domain.StatBlock{AC: 99, MaxHP: 3}}}
	tr = NewToolRouter(srdSession(authored))
	res = call(tr, "get_npc", map[string]any{"npc_id": "g2"})
	if strings.Contains(res.Content, "auto-filled") {
		t.Errorf("authored stat block should not be overridden by SRD:\n%s", res.Content)
	}
	if !strings.Contains(res.Content, "AC 99") {
		t.Errorf("authored AC should be shown:\n%s", res.Content)
	}
}

func TestFormatStatBlockRendersFullFields(t *testing.T) {
	sb := &domain.StatBlock{
		Size: "Medium", Type: "Undead", Alignment: "Lawful Evil",
		AC: 13, MaxHP: 13, HitDice: "2d8+4", Speed: "30 ft.", CR: "1/4", XP: 50,
		DamageVulnerabilities: []string{"bludgeoning"},
		DamageImmunities:      []string{"poison"},
		ConditionImmunities:   []string{"exhaustion", "poisoned"},
		SavingThrows:          []string{"DEX +4"},
		Senses:                []string{"darkvision 60 ft."},
		Reactions:             []domain.Action{{Name: "Parry", Description: "adds 2 to AC"}},
		Source:                "SRD 5.1",
	}
	out := formatStatBlock(sb)
	for _, want := range []string{"Medium Undead, Lawful Evil", "HP 13 (2d8+4)", "CR 1/4 (50 XP)",
		"Damage vulnerabilities: bludgeoning", "Condition immunities: exhaustion, poisoned",
		"Saving throws: DEX +4", "Senses: darkvision 60 ft.", "Reaction: Parry", "[source: SRD 5.1]"} {
		if !strings.Contains(out, want) {
			t.Errorf("formatStatBlock missing %q in:\n%s", want, out)
		}
	}
}
