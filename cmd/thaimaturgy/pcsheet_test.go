package main

import (
	"testing"

	"github.com/theburrowhub/thaimaturgy/internal/domain"
)

func TestBuildPCSheet(t *testing.T) {
	c := domain.NewCharacter("Kael", "Elf", "Wizard")
	c.AddItem(domain.InventoryItem{Name: "Dagger", Quantity: 2, Equipped: true})
	c.AddCondition(domain.ConditionPoisoned)
	c.Notes = "Seeks the lost tome."

	objs := buildPCSheet(c)
	if len(objs) == 0 {
		t.Fatal("buildPCSheet returned no objects")
	}
	// A minimal character (no inventory/conditions/notes) must also render.
	if len(buildPCSheet(domain.NewCharacter("Bob", "Human", "Fighter"))) == 0 {
		t.Error("buildPCSheet returned no objects for a bare character")
	}
}

func TestCleanMarkdown(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want string
	}{
		{"whole-line italic", "_Consulting the oracle…_", "Consulting the oracle…"},
		{"two italics on one line", "_Los susurros_ recorren la _oscuridad_", "_Los susurros_ recorren la _oscuridad_"},
		{"heading", "### 15", "15"},
		{"multi-hash heading", "#### Title", "Title"},
		{"bold stripped", "**bold** text", "bold text"},
		{"code ticks stripped", "run `make check`", "run make check"},
		{"underscores in identifiers untouched", "call get_room then set_flag", "call get_room then set_flag"},
		{"italic keeps leading indentation", "  _note_", "  note"},
		{"plain line untouched", "Just a normal line.", "Just a normal line."},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := cleanMarkdown(tt.in); got != tt.want {
				t.Errorf("cleanMarkdown(%q) = %q, want %q", tt.in, got, tt.want)
			}
		})
	}
}
