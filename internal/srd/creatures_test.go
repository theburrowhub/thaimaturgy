package srd

import "testing"

func TestLookupKnownCreature(t *testing.T) {
	sb, ok := Lookup("goblin")
	if !ok {
		t.Fatal("goblin should be in the SRD subset")
	}
	if sb.AC != 15 || sb.MaxHP != 7 {
		t.Errorf("goblin AC/HP = %d/%d; want 15/7", sb.AC, sb.MaxHP)
	}
	if sb.Source == "" {
		t.Error("SRD stat block should carry a Source for attribution")
	}
	if len(sb.Actions) == 0 {
		t.Error("goblin should have actions")
	}
}

func TestLookupIsCaseAndPluralInsensitive(t *testing.T) {
	if _, ok := Lookup("  GOBLIN "); !ok {
		t.Error("lookup should be case/space-insensitive")
	}
	if _, ok := Lookup("goblins"); !ok {
		t.Error("lookup should resolve a simple plural to the singular")
	}
	if _, ok := Lookup("tarrasque"); ok {
		t.Error("a creature not in the subset must not resolve")
	}
}

func TestLookupReturnsDeepCopy(t *testing.T) {
	sb, _ := Lookup("orc")
	sb.MaxHP = 999 // scalar copy
	// Mutate slice ELEMENTS — the deep copy must not alias the shared table.
	if len(sb.Actions) > 0 {
		sb.Actions[0].Name = "MUTATED"
	}
	if len(sb.Languages) > 0 {
		sb.Languages[0] = "MUTATED"
	}
	again, _ := Lookup("orc")
	if again.MaxHP == 999 {
		t.Error("Lookup must return a scalar copy")
	}
	if len(again.Actions) > 0 && again.Actions[0].Name == "MUTATED" {
		t.Error("Lookup must deep-copy Actions — table entry was corrupted")
	}
	if len(again.Languages) > 0 && again.Languages[0] == "MUTATED" {
		t.Error("Lookup must deep-copy Languages — table entry was corrupted")
	}
}

func TestNamesSortedAndComplete(t *testing.T) {
	names := Names()
	if len(names) == 0 {
		t.Fatal("Names() should not be empty")
	}
	for i := 1; i < len(names); i++ {
		if names[i-1] > names[i] {
			t.Errorf("Names() not sorted at %d: %q > %q", i, names[i-1], names[i])
		}
	}
}

func TestEveryCreatureIsWellFormed(t *testing.T) {
	for _, name := range Names() {
		sb, _ := Lookup(name)
		if sb.AC <= 0 || sb.MaxHP <= 0 {
			t.Errorf("%s: AC/HP must be positive (%d/%d)", name, sb.AC, sb.MaxHP)
		}
		if sb.CR == "" || sb.Source == "" || sb.Speed == "" {
			t.Errorf("%s: missing CR/Source/Speed", name)
		}
		if len(sb.Actions) == 0 {
			t.Errorf("%s: should have at least one action", name)
		}
	}
}
