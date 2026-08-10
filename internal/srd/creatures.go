// Package srd provides a curated subset of the D&D 5e System Reference Document
// (SRD 5.1) monster stat blocks, embedded so the virtual DM can pull a complete
// stat block for a standard creature by name without any authored data.
//
// SRD 5.1 is published by Wizards of the Coast LLC under the Creative Commons
// Attribution 4.0 International License (CC-BY-4.0). See docs/srd-statblocks.md
// for the attribution notice. Only a curated subset of common low-CR creatures is
// embedded; the lookup is the extension point for adding more.
package srd

import (
	"strings"

	"github.com/theburrowhub/thaimaturgy/internal/domain"
)

const sourceSRD = "SRD 5.1 (CC-BY-4.0)"

func ab(str, dex, con, int_, wis, cha int) domain.AbilityScores {
	return domain.AbilityScores{STR: str, DEX: dex, CON: con, INT: int_, WIS: wis, CHA: cha}
}

// creatures maps a normalized creature name to its SRD stat block. Keys are
// lower-cased; lookups normalize the query the same way.
var creatures = map[string]domain.StatBlock{
	"goblin": {
		Size: "Small", Type: "Humanoid (goblinoid)", Alignment: "Neutral Evil",
		AC: 15, MaxHP: 7, HitDice: "2d6", Speed: "30 ft.", Abilities: ab(8, 14, 10, 10, 8, 8),
		CR: "1/4", XP: 50, ProfBonus: 2,
		Skills: []string{"Stealth +6"}, Senses: []string{"darkvision 60 ft.", "passive Perception 9"},
		Languages: []string{"Common", "Goblin"},
		Traits:    []string{"Nimble Escape: can Disengage or Hide as a bonus action on each of its turns."},
		Actions: []domain.Action{
			{Name: "Scimitar", ToHit: "+4", Damage: "1d6+2 slashing"},
			{Name: "Shortbow", ToHit: "+4", Damage: "1d6+2 piercing", Description: "range 80/320 ft."},
		},
		Source: sourceSRD,
	},
	"orc": {
		Size: "Medium", Type: "Humanoid (orc)", Alignment: "Chaotic Evil",
		AC: 13, MaxHP: 15, HitDice: "2d8+6", Speed: "30 ft.", Abilities: ab(16, 12, 16, 7, 11, 10),
		CR: "1/2", XP: 100, ProfBonus: 2,
		Skills: []string{"Intimidation +2"}, Senses: []string{"darkvision 60 ft.", "passive Perception 10"},
		Languages: []string{"Common", "Orc"},
		Traits:    []string{"Aggressive: as a bonus action, can move up to its speed toward a hostile creature it can see."},
		Actions: []domain.Action{
			{Name: "Greataxe", ToHit: "+5", Damage: "1d12+3 slashing"},
			{Name: "Javelin", ToHit: "+5", Damage: "1d6+3 piercing", Description: "range 30/120 ft."},
		},
		Source: sourceSRD,
	},
	"kobold": {
		Size: "Small", Type: "Humanoid (kobold)", Alignment: "Lawful Evil",
		AC: 12, MaxHP: 5, HitDice: "2d6-2", Speed: "30 ft.", Abilities: ab(7, 15, 9, 8, 7, 8),
		CR: "1/8", XP: 25, ProfBonus: 2,
		Senses:    []string{"darkvision 60 ft.", "passive Perception 8"},
		Languages: []string{"Common", "Draconic"},
		Traits: []string{
			"Sunlight Sensitivity: disadvantage on attacks and Perception (sight) in sunlight.",
			"Pack Tactics: advantage on attacks against a creature if an ally is within 5 ft. of it.",
		},
		Actions: []domain.Action{
			{Name: "Dagger", ToHit: "+4", Damage: "1d4+2 piercing"},
			{Name: "Sling", ToHit: "+4", Damage: "1d4+2 bludgeoning", Description: "range 30/120 ft."},
		},
		Source: sourceSRD,
	},
	"skeleton": {
		Size: "Medium", Type: "Undead", Alignment: "Lawful Evil",
		AC: 13, MaxHP: 13, HitDice: "2d8+4", Speed: "30 ft.", Abilities: ab(10, 14, 15, 6, 8, 5),
		CR: "1/4", XP: 50, ProfBonus: 2,
		DamageVulnerabilities: []string{"bludgeoning"},
		DamageImmunities:      []string{"poison"},
		ConditionImmunities:   []string{"exhaustion", "poisoned"},
		Senses:                []string{"darkvision 60 ft.", "passive Perception 9"},
		Languages:             []string{"understands the languages it knew in life but can't speak"},
		Actions: []domain.Action{
			{Name: "Shortsword", ToHit: "+4", Damage: "1d6+2 piercing"},
			{Name: "Shortbow", ToHit: "+4", Damage: "1d6+2 piercing", Description: "range 80/320 ft."},
		},
		Source: sourceSRD,
	},
	"zombie": {
		Size: "Medium", Type: "Undead", Alignment: "Neutral Evil",
		AC: 8, MaxHP: 22, HitDice: "3d8+9", Speed: "20 ft.", Abilities: ab(13, 6, 16, 3, 6, 5),
		CR: "1/4", XP: 50, ProfBonus: 2,
		SavingThrows:        []string{"WIS +0"},
		DamageImmunities:    []string{"poison"},
		ConditionImmunities: []string{"poisoned"},
		Senses:              []string{"darkvision 60 ft.", "passive Perception 8"},
		Languages:           []string{"understands the languages it knew in life but can't speak"},
		Traits:              []string{"Undead Fortitude: if reduced to 0 HP by non-radiant, non-critical damage, makes a CON save (DC 5 + damage) to drop to 1 HP instead."},
		Actions:             []domain.Action{{Name: "Slam", ToHit: "+3", Damage: "1d6+1 bludgeoning"}},
		Source:              sourceSRD,
	},
	"wolf": {
		Size: "Medium", Type: "Beast", Alignment: "Unaligned",
		AC: 13, MaxHP: 11, HitDice: "2d8+2", Speed: "40 ft.", Abilities: ab(12, 15, 12, 3, 12, 6),
		CR: "1/4", XP: 50, ProfBonus: 2,
		Skills: []string{"Perception +3", "Stealth +4"}, Senses: []string{"passive Perception 13"},
		Traits: []string{
			"Keen Hearing and Smell: advantage on Perception checks relying on hearing or smell.",
			"Pack Tactics: advantage on attacks against a creature if an ally is within 5 ft. of it.",
		},
		Actions: []domain.Action{{Name: "Bite", ToHit: "+4", Damage: "2d4+2 piercing", Description: "target DC 11 STR save or be knocked prone."}},
		Source:  sourceSRD,
	},
	"giant rat": {
		Size: "Small", Type: "Beast", Alignment: "Unaligned",
		AC: 12, MaxHP: 7, HitDice: "2d6", Speed: "30 ft.", Abilities: ab(7, 15, 11, 2, 10, 4),
		CR: "1/8", XP: 25, ProfBonus: 2,
		Senses: []string{"darkvision 60 ft.", "passive Perception 10"},
		Traits: []string{
			"Keen Smell: advantage on Perception checks relying on smell.",
			"Pack Tactics: advantage on attacks against a creature if an ally is within 5 ft. of it.",
		},
		Actions: []domain.Action{{Name: "Bite", ToHit: "+4", Damage: "1d4+2 piercing"}},
		Source:  sourceSRD,
	},
	"bandit": {
		Size: "Medium", Type: "Humanoid", Alignment: "Any Non-Lawful",
		AC: 12, MaxHP: 11, HitDice: "2d8+2", Speed: "30 ft.", Abilities: ab(11, 12, 12, 10, 10, 10),
		CR: "1/8", XP: 25, ProfBonus: 2,
		Senses: []string{"passive Perception 10"}, Languages: []string{"any one language (usually Common)"},
		Actions: []domain.Action{
			{Name: "Scimitar", ToHit: "+3", Damage: "1d6+1 slashing"},
			{Name: "Light Crossbow", ToHit: "+3", Damage: "1d8+1 piercing", Description: "range 80/320 ft."},
		},
		Source: sourceSRD,
	},
	"guard": {
		Size: "Medium", Type: "Humanoid", Alignment: "Any",
		AC: 16, MaxHP: 11, HitDice: "2d8+2", Speed: "30 ft.", Abilities: ab(13, 12, 12, 10, 11, 10),
		CR: "1/8", XP: 25, ProfBonus: 2,
		Skills: []string{"Perception +2"}, Senses: []string{"passive Perception 12"},
		Languages: []string{"any one language (usually Common)"},
		Actions:   []domain.Action{{Name: "Spear", ToHit: "+3", Damage: "1d6+1 piercing", Description: "or 1d8+1 if wielded two-handed; thrown range 20/60 ft."}},
		Source:    sourceSRD,
	},
	"bugbear": {
		Size: "Medium", Type: "Humanoid (goblinoid)", Alignment: "Chaotic Evil",
		AC: 16, MaxHP: 27, HitDice: "5d8+5", Speed: "30 ft.", Abilities: ab(15, 14, 13, 8, 11, 9),
		CR: "1", XP: 200, ProfBonus: 2,
		Skills: []string{"Stealth +6", "Survival +2"}, Senses: []string{"darkvision 60 ft.", "passive Perception 10"},
		Languages: []string{"Common", "Goblin"},
		Traits: []string{
			"Brute: a melee weapon deals one extra die of damage when it hits (included).",
			"Surprise Attack: +7 (2d6) damage if it surprises a creature and hits it in the first round.",
		},
		Actions: []domain.Action{
			{Name: "Morningstar", ToHit: "+4", Damage: "2d8+2 piercing (Brute die included)"},
			{Name: "Javelin", ToHit: "+4", Damage: "2d6+2 piercing in melee, 1d6+2 at range", Description: "range 30/120 ft.; the extra melee die is the Brute trait."},
		},
		Source: sourceSRD,
	},
	"hobgoblin": {
		Size: "Medium", Type: "Humanoid (goblinoid)", Alignment: "Lawful Evil",
		AC: 18, MaxHP: 11, HitDice: "2d8+2", Speed: "30 ft.", Abilities: ab(13, 12, 12, 10, 10, 9),
		CR: "1/2", XP: 100, ProfBonus: 2,
		Senses: []string{"darkvision 60 ft.", "passive Perception 10"}, Languages: []string{"Common", "Goblin"},
		Traits: []string{"Martial Advantage: once per turn, +2d6 damage to a creature it hits if an ally is within 5 ft. of the target."},
		Actions: []domain.Action{
			{Name: "Longsword", ToHit: "+3", Damage: "1d8+1 slashing", Description: "or 1d10+1 if wielded two-handed."},
			{Name: "Longbow", ToHit: "+3", Damage: "1d8+1 piercing", Description: "range 150/600 ft."},
		},
		Source: sourceSRD,
	},
	"ogre": {
		Size: "Large", Type: "Giant", Alignment: "Chaotic Evil",
		AC: 11, MaxHP: 59, HitDice: "7d10+21", Speed: "40 ft.", Abilities: ab(19, 8, 16, 5, 7, 7),
		CR: "2", XP: 450, ProfBonus: 2,
		Senses: []string{"darkvision 60 ft.", "passive Perception 8"}, Languages: []string{"Common", "Giant"},
		Actions: []domain.Action{
			{Name: "Greatclub", ToHit: "+6", Damage: "2d8+4 bludgeoning"},
			{Name: "Javelin", ToHit: "+6", Damage: "2d6+4 piercing", Description: "range 30/120 ft."},
		},
		Source: sourceSRD,
	},
	"ghoul": {
		Size: "Medium", Type: "Undead", Alignment: "Chaotic Evil",
		AC: 12, MaxHP: 22, HitDice: "5d8", Speed: "30 ft.", Abilities: ab(13, 15, 10, 7, 10, 6),
		CR: "1", XP: 200, ProfBonus: 2,
		DamageImmunities:    []string{"poison"},
		ConditionImmunities: []string{"charmed", "exhaustion", "poisoned"},
		Senses:              []string{"darkvision 60 ft.", "passive Perception 10"}, Languages: []string{"Common"},
		Actions: []domain.Action{
			{Name: "Bite", ToHit: "+2", Damage: "2d6 piercing"},
			{Name: "Claws", ToHit: "+4", Damage: "2d4+2 slashing", Description: "if the target is a non-elf, non-undead creature it must succeed on a DC 10 CON save or be paralyzed for 1 minute (repeat save at end of each turn)."},
		},
		Source: sourceSRD,
	},
	"giant spider": {
		Size: "Large", Type: "Beast", Alignment: "Unaligned",
		AC: 14, MaxHP: 26, HitDice: "4d10+4", Speed: "30 ft., climb 30 ft.", Abilities: ab(14, 16, 12, 2, 11, 4),
		CR: "1", XP: 200, ProfBonus: 2,
		Skills: []string{"Stealth +7"}, Senses: []string{"blindsight 10 ft.", "darkvision 60 ft.", "passive Perception 10"},
		Traits: []string{
			"Spider Climb: can climb difficult surfaces, including upside down, without a check.",
			"Web Sense: while in contact with a web, knows the location of any other creature in contact with the same web.",
			"Web Walker: ignores movement restrictions caused by webbing.",
		},
		Actions: []domain.Action{
			{Name: "Bite", ToHit: "+5", Damage: "1d8+3 piercing plus 2d8 poison", Description: "DC 11 CON save; half poison damage on a success, and if reduced to 0 HP the target is stable but poisoned and paralyzed for 1 hour."},
			{Name: "Web (Recharge 5-6)", ToHit: "+5", Description: "ranged 30/60 ft., one creature; on a hit the target is restrained by webbing. The restrained target can use its action for a DC 12 Strength check to break free. The webbing can also be attacked and destroyed (AC 10; 5 HP; vulnerable to fire; immune to bludgeoning, poison, and psychic)."},
		},
		Source: sourceSRD,
	},
	"commoner": {
		Size: "Medium", Type: "Humanoid", Alignment: "Any",
		AC: 10, MaxHP: 4, HitDice: "1d8", Speed: "30 ft.", Abilities: ab(10, 10, 10, 10, 10, 10),
		CR: "0", XP: 10, ProfBonus: 2,
		Senses: []string{"passive Perception 10"}, Languages: []string{"any one language (usually Common)"},
		Actions: []domain.Action{{Name: "Club", ToHit: "+2", Damage: "1d4 bludgeoning"}},
		Source:  sourceSRD,
	},
}

// normalize lower-cases and trims a creature name for lookup.
func normalize(name string) string {
	return strings.Join(strings.Fields(strings.ToLower(strings.TrimSpace(name))), " ")
}

// Lookup returns the SRD stat block for a creature by name (case-insensitive),
// trying an exact match first and then a singular form (trailing "s" removed), so
// "goblins" resolves to "goblin". The returned block is a DEEP copy — every slice
// is cloned — so the caller may freely mutate it without corrupting the shared,
// process-wide table or racing another lookup.
func Lookup(name string) (domain.StatBlock, bool) {
	key := normalize(name)
	sb, ok := creatures[key]
	if !ok && strings.HasSuffix(key, "s") {
		sb, ok = creatures[strings.TrimSuffix(key, "s")]
	}
	if !ok {
		return domain.StatBlock{}, false
	}
	return deepCopy(sb), true
}

// deepCopy clones every slice in a stat block so the returned value shares no
// backing arrays with the table entry.
func deepCopy(sb domain.StatBlock) domain.StatBlock {
	cp := sb
	cp.SavingThrows = append([]string(nil), sb.SavingThrows...)
	cp.Skills = append([]string(nil), sb.Skills...)
	cp.Senses = append([]string(nil), sb.Senses...)
	cp.Languages = append([]string(nil), sb.Languages...)
	cp.DamageResistances = append([]string(nil), sb.DamageResistances...)
	cp.DamageImmunities = append([]string(nil), sb.DamageImmunities...)
	cp.DamageVulnerabilities = append([]string(nil), sb.DamageVulnerabilities...)
	cp.ConditionImmunities = append([]string(nil), sb.ConditionImmunities...)
	cp.Traits = append([]string(nil), sb.Traits...)
	cp.Actions = append([]domain.Action(nil), sb.Actions...)
	cp.Reactions = append([]domain.Action(nil), sb.Reactions...)
	cp.LegendaryActions = append([]domain.Action(nil), sb.LegendaryActions...)
	return cp
}

// Names returns the sorted list of creature names available in the embedded SRD
// subset (for listing / help).
func Names() []string {
	out := make([]string, 0, len(creatures))
	for k := range creatures {
		out = append(out, k)
	}
	// insertion-order-independent, cheap sort
	for i := 1; i < len(out); i++ {
		for j := i; j > 0 && out[j-1] > out[j]; j-- {
			out[j-1], out[j] = out[j], out[j-1]
		}
	}
	return out
}
