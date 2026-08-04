package domain

import "testing"

func TestGenerateCharacterDwarfCleric(t *testing.T) {
	c := GenerateCharacter("", "Dwarf", "Cleric", 1)
	if c.Race != "Dwarf" || c.Class != "Cleric" || c.Level != 1 {
		t.Fatalf("identity wrong: %s %s L%d", c.Race, c.Class, c.Level)
	}
	// Cleric puts the top score (15) in WIS; Dwarf adds +2 CON to the 14.
	if c.Abilities.WIS != 15 {
		t.Errorf("WIS = %d, want 15", c.Abilities.WIS)
	}
	if c.Abilities.CON != 16 {
		t.Errorf("CON = %d, want 16 (14 + Dwarf +2)", c.Abilities.CON)
	}
	if c.MaxHP != 11 { // d8 + CON mod (+3)
		t.Errorf("MaxHP = %d, want 11", c.MaxHP)
	}
	if c.CurrentHP != c.MaxHP {
		t.Errorf("CurrentHP = %d, want %d", c.CurrentHP, c.MaxHP)
	}
	if c.AC != 14 { // medium armor base 14 + min(dexMod 0, 2)
		t.Errorf("AC = %d, want 14", c.AC)
	}
	if c.Speed != 25 { // dwarf
		t.Errorf("Speed = %d, want 25", c.Speed)
	}
	if c.ProficiencyBonus != 2 {
		t.Errorf("Prof = %d, want 2", c.ProficiencyBonus)
	}
	if c.Name == "" {
		t.Error("expected a sample name for an empty name")
	}
}

func TestProficiencyBonusForLevel(t *testing.T) {
	cases := map[int]int{1: 2, 4: 2, 5: 3, 8: 3, 9: 4, 13: 5, 17: 6}
	for lvl, want := range cases {
		if got := ProficiencyBonusForLevel(lvl); got != want {
			t.Errorf("ProficiencyBonusForLevel(%d) = %d, want %d", lvl, got, want)
		}
	}
}

func TestGenerateCharacterLevelScaling(t *testing.T) {
	l1 := GenerateCharacter("A", "Human", "Fighter", 1)
	l5 := GenerateCharacter("A", "Human", "Fighter", 5)
	if l5.ProficiencyBonus != 3 {
		t.Errorf("level 5 prof = %d, want 3", l5.ProficiencyBonus)
	}
	if l5.MaxHP <= l1.MaxHP {
		t.Errorf("HP should grow with level: L1=%d L5=%d", l1.MaxHP, l5.MaxHP)
	}
	if l5.Level != 5 {
		t.Errorf("level = %d, want 5", l5.Level)
	}
}

func TestNormalizeFallbacks(t *testing.T) {
	if GenerateCharacter("X", "Griffonkin", "Astronaut", 0).Level != 1 {
		t.Error("level < 1 should default to 1")
	}
	if got := NormalizeRace("orc"); got != "Human" {
		t.Errorf("unknown race → %q, want Human", got)
	}
	if got := NormalizeClass("mage"); got != "Fighter" {
		t.Errorf("unknown class → %q, want Fighter", got)
	}
	if got := NormalizeRace("half-orc"); got != "Half-Orc" {
		t.Errorf("race casing = %q, want Half-Orc", got)
	}
}

func TestDefaultParty(t *testing.T) {
	party := DefaultParty()
	if len(party) < 3 {
		t.Fatalf("default party too small: %d", len(party))
	}
	races, classes := map[string]bool{}, map[string]bool{}
	for _, c := range party {
		if c.Level != 1 {
			t.Errorf("%s level = %d, want 1", c.Name, c.Level)
		}
		races[c.Race] = true
		classes[c.Class] = true
	}
	if len(races) < 3 || len(classes) < 3 {
		t.Errorf("party should be heterogeneous: %d races, %d classes", len(races), len(classes))
	}
}
