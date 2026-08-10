package dmbook

import (
	"strings"
	"testing"

	"github.com/theburrowhub/thaimaturgy/internal/domain"
)

// TestMarkdownRendersExpandedStatBlock ensures the DM book (#26) exports the full
// stat block, not just the original core fields.
func TestMarkdownRendersExpandedStatBlock(t *testing.T) {
	adv := &domain.Adventure{
		SchemaVersion: domain.SchemaVersion, ID: "b", Title: "Book",
		NPCs: []domain.NPC{{
			ID: "wight", Name: "Wight",
			StatBlock: &domain.StatBlock{
				Size: "Medium", Type: "Undead", Alignment: "Neutral Evil",
				AC: 14, MaxHP: 45, HitDice: "6d8+18", Speed: "30 ft.", CR: "3", XP: 700, ProfBonus: 2,
				SavingThrows:        []string{"WIS +3"},
				DamageResistances:   []string{"necrotic"},
				DamageImmunities:    []string{"poison"},
				ConditionImmunities: []string{"exhaustion", "poisoned"},
				Senses:              []string{"darkvision 60 ft."},
				Languages:           []string{"the languages it knew in life"},
				Reactions:           []domain.Action{{Name: "Parry"}},
				LegendaryActions:    []domain.Action{{Name: "Move"}},
				Source:              "SRD 5.1",
			},
		}},
	}
	md := Markdown(adv)
	for _, want := range []string{
		"Medium Undead, Neutral Evil", "HP 45 (6d8+18)", "700 XP", "Prof +2",
		"Saving throws: WIS +3", "Damage resistances: necrotic", "Damage immunities: poison",
		"Condition immunities: exhaustion, poisoned", "Senses: darkvision 60 ft.",
		"Languages: the languages it knew in life", "*Reaction:* Parry",
		"*Legendary Action:* Move", "*Source:* SRD 5.1",
	} {
		if !strings.Contains(md, want) {
			t.Errorf("DM book missing %q", want)
		}
	}
}
