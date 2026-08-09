package main

import (
	"testing"

	"github.com/theburrowhub/thaimaturgy/internal/domain"
)

func TestAtoiDefault(t *testing.T) {
	if atoiDefault("42", 0) != 42 {
		t.Error("valid int not parsed")
	}
	if atoiDefault("  -7 ", 0) != -7 {
		t.Error("negative int with spaces not parsed")
	}
	if atoiDefault("abc", 9) != 9 {
		t.Error("invalid int should fall back to default")
	}
}

func TestParseCSVList(t *testing.T) {
	got := parseCSVList("Common, Elvish\nDwarvish ,, ")
	want := []string{"Common", "Elvish", "Dwarvish"}
	if len(got) != len(want) {
		t.Fatalf("got %v; want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("got[%d]=%q; want %q", i, got[i], want[i])
		}
	}
}

func TestInventoryRoundTrip(t *testing.T) {
	items := []domain.InventoryItem{
		{Name: "Longsword", Quantity: 1, Equipped: true},
		{Name: "Torch", Quantity: 5},
	}
	parsed := parseInventoryLines(formatInventoryLines(items))
	if len(parsed) != 2 {
		t.Fatalf("expected 2 items, got %d: %+v", len(parsed), parsed)
	}
	if parsed[0].Name != "Longsword" || !parsed[0].Equipped || parsed[0].Quantity != 1 {
		t.Errorf("item 0 wrong: %+v", parsed[0])
	}
	if parsed[1].Name != "Torch" || parsed[1].Quantity != 5 {
		t.Errorf("item 1 wrong: %+v", parsed[1])
	}
}

func TestParseInventoryEdgeCases(t *testing.T) {
	got := parseInventoryLines("Rope of Climbing x2 [E]\n\n  \nDagger")
	if len(got) != 2 {
		t.Fatalf("expected 2 items, got %d: %+v", len(got), got)
	}
	if got[0].Name != "Rope of Climbing" || got[0].Quantity != 2 || !got[0].Equipped {
		t.Errorf("multi-word + qty + equipped wrong: %+v", got[0])
	}
	if got[1].Name != "Dagger" || got[1].Quantity != 1 {
		t.Errorf("bare item wrong: %+v", got[1])
	}
}

func TestSpellRoundTrip(t *testing.T) {
	spells := []domain.Spell{
		{Name: "Fire Bolt", Level: 0},
		{Name: "Fireball", Level: 3, Prepared: true},
	}
	parsed := parseSpellLines(formatSpellLines(spells))
	if len(parsed) != 2 {
		t.Fatalf("expected 2 spells, got %d: %+v", len(parsed), parsed)
	}
	if parsed[0].Name != "Fire Bolt" || parsed[0].Level != 0 {
		t.Errorf("cantrip wrong: %+v", parsed[0])
	}
	if parsed[1].Name != "Fireball" || parsed[1].Level != 3 || !parsed[1].Prepared {
		t.Errorf("leveled prepared spell wrong: %+v", parsed[1])
	}
}

func TestSlotSpecRoundTrip(t *testing.T) {
	var slots domain.SpellSlots
	slots.Max[0] = 4 // L1
	slots.Max[2] = 2 // L3
	parsed := parseSlotSpec(formatSlotSpec(slots))
	if parsed.MaxAt(1) != 4 || parsed.MaxAt(3) != 2 || parsed.MaxAt(2) != 0 {
		t.Errorf("slot spec round-trip wrong: %+v", parsed)
	}
	// Out-of-range levels are ignored.
	if got := parseSlotSpec("0:9, 12:3, 2:1"); got.MaxAt(2) != 1 || got != mustOnlyLevel2(1) {
		t.Errorf("out-of-range levels not ignored: %+v", got)
	}
}

func mustOnlyLevel2(n int) domain.SpellSlots {
	var s domain.SpellSlots
	s.Max[1] = n
	return s
}

func TestAbilityFromString(t *testing.T) {
	cases := map[string]domain.Ability{
		"str": domain.STR, "DEX": domain.DEX, " con ": domain.CON,
		"INT": domain.INT, "wis": domain.WIS, "CHA": domain.CHA, "??": domain.INT,
	}
	for in, want := range cases {
		if got := abilityFromString(in); got != want {
			t.Errorf("abilityFromString(%q) = %v; want %v", in, got, want)
		}
	}
}
