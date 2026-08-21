package main

import (
	"strconv"
	"strings"
	"testing"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/test"
	"fyne.io/fyne/v2/widget"

	"github.com/theburrowhub/thaimaturgy/internal/domain"
)

// TestSheetEditorFormRoundTrip guards the local/remote editor parity: building
// the shared form from a fully populated caster and immediately collect()-ing it
// must faithfully reproduce every exposed field (abilities, saves, skills,
// conditions, inventory, spells and preserved spent slots).
func TestSheetEditorFormRoundTrip(t *testing.T) {
	test.NewApp() // a test driver so widget construction is safe headless

	base := domain.GenerateCharacter("Mage", "Elf", "Wizard", 5)
	base.AddSpell(domain.Spell{Name: "Fireball", Level: 3, Prepared: true, School: "Evocation"})
	base.UseSpellSlot(1) // spend a slot the form doesn't expose directly
	base.AddItem(domain.InventoryItem{Name: "Staff", Quantity: 1, Equipped: true, Weight: 4})
	base.AddCondition(domain.ConditionPoisoned)
	base.Normalize()

	_, _, collect := sheetEditorForm(*base)
	edited, ok := collect()
	if !ok {
		t.Fatal("collect() reported a validation error on an unchanged, valid sheet")
	}
	// Unexposed inventory metadata (Weight) must survive an open→save cycle.
	if w := weightOf(edited.Inventory, "Staff"); w != 4 {
		t.Errorf("item Weight lost in round-trip: got %v want 4", w)
	}

	if edited.Spellcasting == nil {
		t.Fatal("spellcasting dropped in round-trip")
	}
	if got := edited.Spellcasting.Slots.Used[0]; got != base.Spellcasting.Slots.Used[0] {
		t.Errorf("spent 1st-level slot not preserved: used=%d want %d", got, base.Spellcasting.Slots.Used[0])
	}
	if !hasSpell(edited.Spellcasting.Spells, "Fireball", true) {
		t.Error("prepared Fireball lost in round-trip")
	}
	if !hasEquipped(edited.Inventory, "Staff") {
		t.Error("equipped Staff lost in round-trip")
	}
	if !hasCondition(edited.Conditions, domain.ConditionPoisoned) {
		t.Error("Poisoned condition lost in round-trip")
	}
	if !edited.SaveProficient(domain.INT) || !edited.SaveProficient(domain.WIS) {
		t.Error("wizard INT/WIS save proficiencies lost in round-trip")
	}
}

// TestSheetEditorFormRejectsMalformedNumber guards that a malformed numeric field
// is reported (dialog kept open via the status label) rather than silently
// falling back to the baseline and reporting a misleading partial success.
func TestSheetEditorFormRejectsMalformedNumber(t *testing.T) {
	test.NewApp()

	base := domain.GenerateCharacter("Bob", "Human", "Fighter", 3)
	content, status, collect := sheetEditorForm(*base)
	if _, ok := collect(); !ok {
		t.Fatal("valid baseline should collect cleanly")
	}
	// Corrupt the first numeric entry (one whose current text is an integer — an
	// ability score, reachable in the form's grid) with a malformed value; collect()
	// must reject it instead of silently reverting to the baseline.
	corrupted := false
	for _, e := range findEntries(content) {
		if _, err := strconv.Atoi(strings.TrimSpace(e.Text)); err == nil {
			e.SetText("12x")
			corrupted = true
			break
		}
	}
	if !corrupted {
		t.Fatal("expected at least one numeric entry to corrupt")
	}
	if _, ok := collect(); ok {
		t.Error("collect() accepted a malformed numeric field instead of rejecting it")
	}
	if strings.TrimSpace(status.Text) == "" {
		t.Error("expected a field-specific validation message in the status label")
	}
}

// findEntries walks a widget tree and collects every *widget.Entry reachable
// through containers (used by the malformed-number test).
func findEntries(obj fyne.CanvasObject) []*widget.Entry {
	switch w := obj.(type) {
	case *widget.Entry:
		return []*widget.Entry{w}
	case *fyne.Container:
		var out []*widget.Entry
		for _, c := range w.Objects {
			out = append(out, findEntries(c)...)
		}
		return out
	}
	return nil
}

// TestMergeInventoryMetadataDuplicateNames guards that two items sharing a name
// but with different weights each keep their own weight across an unchanged
// open→save round trip (a name-keyed map would corrupt them to the last weight).
func TestMergeInventoryMetadataDuplicateNames(t *testing.T) {
	prev := []domain.InventoryItem{
		{Name: "Potion", Quantity: 1, Weight: 0.5},
		{Name: "Potion", Quantity: 1, Weight: 2.0},
		{Name: "Rope", Quantity: 1, Weight: 5},
	}
	// An unchanged round-trip: format then parse reproduces the same lines and order.
	merged := mergeInventoryMetadata(parseInventoryLines(formatInventoryLines(prev)), prev)
	if len(merged) != 3 {
		t.Fatalf("got %d items, want 3", len(merged))
	}
	if merged[0].Weight != 0.5 || merged[1].Weight != 2.0 {
		t.Errorf("duplicate-name weights not preserved by occurrence: %v / %v", merged[0].Weight, merged[1].Weight)
	}
	if merged[2].Weight != 5 {
		t.Errorf("Rope weight = %v; want 5", merged[2].Weight)
	}
}

func weightOf(items []domain.InventoryItem, name string) float64 {
	for _, it := range items {
		if it.Name == name {
			return it.Weight
		}
	}
	return -1
}

func hasSpell(ss []domain.Spell, name string, prepared bool) bool {
	for _, s := range ss {
		if s.Name == name && s.Prepared == prepared {
			return true
		}
	}
	return false
}

func hasEquipped(items []domain.InventoryItem, name string) bool {
	for _, it := range items {
		if it.Name == name && it.Equipped {
			return true
		}
	}
	return false
}

func hasCondition(cs []domain.Condition, want domain.Condition) bool {
	for _, c := range cs {
		if c == want {
			return true
		}
	}
	return false
}

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
	parsed, err := parseSlotSpec(formatSlotSpec(slots))
	if err != nil {
		t.Fatalf("round-trip errored: %v", err)
	}
	if parsed.MaxAt(1) != 4 || parsed.MaxAt(3) != 2 || parsed.MaxAt(2) != 0 {
		t.Errorf("slot spec round-trip wrong: %+v", parsed)
	}
}

func TestParseSlotSpecRejectsMalformed(t *testing.T) {
	// A malformed count must be rejected (not silently zeroed), and out-of-range
	// or non-numeric levels must error too.
	for _, in := range []string{"1:4, 2:x", "0:9", "12:3", "abc", "1:-2"} {
		if _, err := parseSlotSpec(in); err == nil {
			t.Errorf("parseSlotSpec(%q) should have errored", in)
		}
	}
	// Empty and whitespace-only are valid (no slots).
	for _, in := range []string{"", "  ,  \n"} {
		if _, err := parseSlotSpec(in); err != nil {
			t.Errorf("parseSlotSpec(%q) should be valid, got %v", in, err)
		}
	}
}

func TestMergeSlotUsagePreservesSpent(t *testing.T) {
	prev := &domain.Spellcasting{}
	prev.Slots.Max[0] = 4
	prev.Slots.Used[0] = 3
	prev.Slots.Max[1] = 3
	prev.Slots.Used[1] = 2

	var newMax domain.SpellSlots
	newMax.Max[0] = 4 // unchanged → keep 3 used
	newMax.Max[1] = 1 // lowered below used → clamp used to 1
	merged := mergeSlotUsage(newMax, prev)
	if merged.Used[0] != 3 {
		t.Errorf("L1 used = %d; want preserved 3", merged.Used[0])
	}
	if merged.Used[1] != 1 {
		t.Errorf("L2 used = %d; want clamped to new max 1", merged.Used[1])
	}
}

func TestMergeSpellMetadataPreserved(t *testing.T) {
	prev := []domain.Spell{{Name: "Fireball", Level: 3, School: "Evocation", Description: "boom"}}
	edited := []domain.Spell{{Name: "fireball", Level: 3, Prepared: true}} // form drops school/desc
	merged := mergeSpellMetadata(edited, prev)
	if merged[0].School != "Evocation" || merged[0].Description != "boom" {
		t.Errorf("spell metadata not preserved: %+v", merged[0])
	}
	if !merged[0].Prepared {
		t.Error("edited prepared flag should be kept")
	}
}

func TestFeatureRoundTrip(t *testing.T) {
	traits := []domain.Trait{
		{Name: "Darkvision", Source: "Race", Description: "60 ft"},
		{Name: "Second Wind", Source: "Class"},
		{Name: "Lucky"},
		// A multi-line description containing a pipe must survive the round-trip
		// and not be split into extra traits (Heimdallm review).
		{Name: "Spellcasting", Source: "Class", Description: "Line one.\nLine two | with a pipe.\nLine three."},
	}
	parsed := parseFeatureLines(formatFeatureLines(traits))
	if len(parsed) != len(traits) {
		t.Fatalf("expected %d traits, got %d: %+v", len(traits), len(parsed), parsed)
	}
	for i := range traits {
		if parsed[i] != traits[i] {
			t.Errorf("trait %d not preserved:\n got %+v\nwant %+v", i, parsed[i], traits[i])
		}
	}
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
