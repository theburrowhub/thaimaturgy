package domain

import (
	"encoding/json"
	"testing"
)

func TestSaveBonusAndProficiency(t *testing.T) {
	c := NewCharacter("Test", "Human", "Fighter")
	c.ProficiencyBonus = 3
	c.Abilities.Set(CON, 16) // +3
	c.Abilities.Set(DEX, 14) // +2
	c.SetSaveProficient(CON, true)

	if got := c.SaveBonus(CON); got != 6 { // 3 (mod) + 3 (prof)
		t.Errorf("CON save = %d; want 6", got)
	}
	if got := c.SaveBonus(DEX); got != 2 { // 2 (mod), not proficient
		t.Errorf("DEX save = %d; want 2", got)
	}
	c.SetSaveProficient(CON, false)
	if c.SaveProficient(CON) {
		t.Error("CON save proficiency should have been removed")
	}
}

func TestSpellSlotsUseRestore(t *testing.T) {
	c := NewCharacter("Mage", "Elf", "Wizard")
	c.Spellcasting = &Spellcasting{Ability: INT}
	c.Spellcasting.Slots.Max[0] = 2 // two 1st-level slots

	if c.SpellSlotsRemaining(1) != 2 {
		t.Fatalf("remaining = %d; want 2", c.SpellSlotsRemaining(1))
	}
	if !c.UseSpellSlot(1) || !c.UseSpellSlot(1) {
		t.Fatal("expected two successful slot uses")
	}
	if c.UseSpellSlot(1) {
		t.Error("third use should fail: no slots left")
	}
	if c.SpellSlotsRemaining(1) != 0 {
		t.Errorf("remaining = %d; want 0", c.SpellSlotsRemaining(1))
	}
	if !c.RestoreSpellSlot(1) {
		t.Error("restoring a spent slot should report a change")
	}
	if c.SpellSlotsRemaining(1) != 1 {
		t.Errorf("after restore remaining = %d; want 1", c.SpellSlotsRemaining(1))
	}
	// Restoring again to full works; a further restore is a no-op and reports so.
	if !c.RestoreSpellSlot(1) {
		t.Error("restoring the second spent slot should report a change")
	}
	if c.RestoreSpellSlot(1) {
		t.Error("restoring at full slots should report NO change (no-op)")
	}
	// Invalid level and non-caster are safe no-ops.
	if c.UseSpellSlot(0) || c.UseSpellSlot(10) {
		t.Error("invalid spell levels must not consume a slot")
	}
	if NewCharacter("Bob", "Human", "Fighter").UseSpellSlot(1) {
		t.Error("a non-caster must not be able to use a spell slot")
	}
}

func TestSpellbookOps(t *testing.T) {
	c := NewCharacter("Mage", "Elf", "Wizard")
	c.AddSpell(Spell{Name: "Magic Missile", Level: 1})
	c.AddSpell(Spell{Name: "Shield", Level: 1})
	if c.Spellcasting == nil || len(c.Spellcasting.Spells) != 2 {
		t.Fatalf("expected 2 spells, got %+v", c.Spellcasting)
	}
	if !c.SetSpellPrepared("shield", true) || !c.Spellcasting.Spells[1].Prepared {
		t.Error("SetSpellPrepared should mark Shield prepared (case-insensitive)")
	}
	// Adding by same name updates rather than duplicates.
	c.AddSpell(Spell{Name: "shield", Level: 1, School: "Abjuration"})
	if len(c.Spellcasting.Spells) != 2 {
		t.Errorf("re-adding by name should update, got %d spells", len(c.Spellcasting.Spells))
	}
	if !c.RemoveSpell("Magic Missile") || len(c.Spellcasting.Spells) != 1 {
		t.Errorf("RemoveSpell failed: %+v", c.Spellcasting.Spells)
	}
}

func TestLongRestRestoresSpellSlots(t *testing.T) {
	c := GenerateCharacter("Mage", "Elf", "Wizard", 5)
	if c.Spellcasting == nil {
		t.Fatal("a level-5 wizard should have spellcasting")
	}
	c.UseSpellSlot(1)
	c.UseSpellSlot(3)
	c.LongRest()
	for lvl := 1; lvl <= 9; lvl++ {
		if c.SpellSlotsRemaining(lvl) != c.Spellcasting.Slots.MaxAt(lvl) {
			t.Errorf("level %d slots not fully restored after long rest", lvl)
		}
	}
}

func TestNormalizeClamps(t *testing.T) {
	c := NewCharacter("X", "Human", "Fighter")
	c.MaxHP = 20
	c.CurrentHP = 999
	c.TempHP = -5
	c.Gold = -3
	c.Level = 0
	c.HitDiceUsed = 50
	c.Inventory = []InventoryItem{{Name: "Torch", Quantity: 0}}
	c.Spellcasting = &Spellcasting{Ability: INT}
	c.Spellcasting.Slots.Max[0] = 2
	c.Spellcasting.Slots.Used[0] = 9
	c.Normalize()

	if c.CurrentHP != 20 {
		t.Errorf("CurrentHP = %d; want clamped to 20", c.CurrentHP)
	}
	if c.TempHP != 0 || c.Gold != 0 {
		t.Errorf("negative temp/gold not clamped: temp=%d gold=%d", c.TempHP, c.Gold)
	}
	if c.Level != 1 {
		t.Errorf("Level = %d; want 1", c.Level)
	}
	if c.HitDiceUsed != c.HitDiceMax() {
		t.Errorf("HitDiceUsed = %d; want clamped to max %d", c.HitDiceUsed, c.HitDiceMax())
	}
	if c.Inventory[0].Quantity != 1 {
		t.Errorf("item quantity = %d; want 1", c.Inventory[0].Quantity)
	}
	if c.Spellcasting.Slots.Used[0] != 2 {
		t.Errorf("slots used = %d; want clamped to max 2", c.Spellcasting.Slots.Used[0])
	}
}

func TestGenerateCharacterCaster(t *testing.T) {
	w := GenerateCharacter("W", "Elf", "Wizard", 1)
	if w.Spellcasting == nil {
		t.Fatal("level-1 wizard should be a caster")
	}
	if w.Spellcasting.Ability != INT {
		t.Errorf("wizard casting ability = %v; want INT", w.Spellcasting.Ability)
	}
	if w.Spellcasting.Slots.MaxAt(1) != 2 {
		t.Errorf("level-1 wizard 1st-level slots = %d; want 2", w.Spellcasting.Slots.MaxAt(1))
	}
	// Save DC = 8 + prof(2) + INT mod.
	wantDC := 8 + w.ProficiencyBonus + Modifier(w.Abilities.INT)
	if w.Spellcasting.SaveDC != wantDC {
		t.Errorf("spell save DC = %d; want %d", w.Spellcasting.SaveDC, wantDC)
	}
	if !w.SaveProficient(INT) || !w.SaveProficient(WIS) {
		t.Error("wizard should be proficient in INT and WIS saves")
	}
	if len(w.Languages) == 0 {
		t.Error("generated character should have starting languages")
	}

	// A fighter is not a caster.
	if f := GenerateCharacter("F", "Human", "Fighter", 3); f.Spellcasting != nil {
		t.Error("fighter should not have a spellcasting block")
	}
	// A level-5 wizard has a 3rd-level slot; a warlock uses pact slots.
	if GenerateCharacter("W5", "Human", "Wizard", 5).Spellcasting.Slots.MaxAt(3) != 2 {
		t.Error("level-5 wizard should have two 3rd-level slots")
	}
	wl := GenerateCharacter("WL", "Tiefling", "Warlock", 3)
	if wl.Spellcasting.Slots.MaxAt(2) != 2 {
		t.Errorf("level-3 warlock should have two pact (2nd-level) slots, got %d", wl.Spellcasting.Slots.MaxAt(2))
	}
}

func TestGenerateCharacterWithAbilities(t *testing.T) {
	// Base scores (pre-racial); a Dwarf gets CON +2 on top.
	base := AbilityScores{STR: 15, DEX: 12, CON: 14, INT: 10, WIS: 13, CHA: 8}
	c := GenerateCharacterWithAbilities("Thrain", "Dwarf", "Fighter", 1, base)
	if c.Abilities.STR != 15 {
		t.Errorf("STR = %d; want the provided 15", c.Abilities.STR)
	}
	if c.Abilities.CON != 16 { // 14 + Dwarf racial +2
		t.Errorf("CON = %d; want 16 (14 + racial)", c.Abilities.CON)
	}
	// HP is derived from the resulting CON modifier (+3): d10 + 3 = 13 at L1.
	if c.MaxHP != 13 {
		t.Errorf("MaxHP = %d; want 13 (d10 + CON mod)", c.MaxHP)
	}
	if c.MaxHP != c.CurrentHP {
		t.Error("a fresh character should start at full HP")
	}
	// Saving throws and (non-)spellcasting still come from the class.
	if !c.SaveProficient(STR) || !c.SaveProficient(CON) {
		t.Error("fighter should have STR and CON save proficiencies")
	}
	if c.Spellcasting != nil {
		t.Error("a fighter must not have a spellcasting block")
	}
}

func TestCharacterJSONRoundTripWithSpells(t *testing.T) {
	c := GenerateCharacter("Mage", "Elf", "Wizard", 3)
	c.AddSpell(Spell{Name: "Fireball", Level: 3, Prepared: true})
	c.Inspiration = true
	b, err := json.Marshal(c)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var loaded Character
	if err := json.Unmarshal(b, &loaded); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if loaded.Spellcasting == nil || len(loaded.Spellcasting.Spells) == 0 {
		t.Fatal("spellcasting lost across round-trip")
	}
	if !loaded.Inspiration {
		t.Error("inspiration lost across round-trip")
	}
	if loaded.Spellcasting.Spells[len(loaded.Spellcasting.Spells)-1].Name != "Fireball" {
		t.Error("spellbook content changed across round-trip")
	}
}
