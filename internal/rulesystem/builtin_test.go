package rulesystem_test

import (
	"strings"
	"testing"

	"github.com/theburrowhub/thaimaturgy/internal/rulesystem"
	_ "github.com/theburrowhub/thaimaturgy/internal/rulesystem/profiles"
)

func TestBuiltinIDs(t *testing.T) {
	ids := rulesystem.BuiltinIDs()
	if len(ids) != 3 {
		t.Fatalf("expected 3 builtins, got %d", len(ids))
	}
}

func TestBuiltinDnD5e(t *testing.T) {
	p, err := rulesystem.Builtin("dnd5e")
	if err != nil {
		t.Fatal(err)
	}
	if p.ID != "dnd5e" {
		t.Fatalf("id=%q", p.ID)
	}
	if len(p.Attributes) != 6 {
		t.Fatalf("attributes=%d want 6", len(p.Attributes))
	}
	if len(p.Skills) != 18 {
		t.Fatalf("skills=%d want 18", len(p.Skills))
	}
	if len(p.Conditions) != 15 {
		t.Fatalf("conditions=%d want 15", len(p.Conditions))
	}
	if len(p.Workflows) < 8 {
		t.Fatalf("workflows=%d want >=8", len(p.Workflows))
	}
	if len(p.Tools) < 15 {
		t.Fatalf("tools=%d want >=15", len(p.Tools))
	}
	if len(p.Chapters) < 4 {
		t.Fatalf("chapters=%d want >=4", len(p.Chapters))
	}
}

func TestBuiltinD100(t *testing.T) {
	p, err := rulesystem.Builtin("d100")
	if err != nil {
		t.Fatal(err)
	}
	if p.Dice.Primary != "d100" {
		t.Fatalf("dice=%q", p.Dice.Primary)
	}
	if len(p.Workflows) < 5 {
		t.Fatalf("workflows=%d", len(p.Workflows))
	}
}

func TestBuiltinSavageWorlds(t *testing.T) {
	p, err := rulesystem.Builtin("savage_worlds")
	if err != nil {
		t.Fatal(err)
	}
	if !p.Dice.Exploding {
		t.Fatal("expected exploding dice")
	}
	found := false
	for _, c := range p.Conditions {
		if c.ID == "shaken" {
			found = true
			break
		}
	}
	if !found {
		t.Fatal("missing shaken condition")
	}
}

func TestListBuiltins(t *testing.T) {
	all := rulesystem.ListBuiltins()
	if len(all) != 3 {
		t.Fatalf("got %d packs", len(all))
	}
}

func TestInspectReport(t *testing.T) {
	p, _ := rulesystem.Builtin("dnd5e")
	report := rulesystem.InspectReport(p)
	if !strings.Contains(report, "Dungeons & Dragons") {
		t.Fatalf("unexpected report: %s", report)
	}
}

func TestGeneratePack(t *testing.T) {
	p, err := rulesystem.Generate(rulesystem.GenerateOptions{TemplateID: "dnd5e"})
	if err != nil {
		t.Fatal(err)
	}
	if p == nil {
		t.Fatal("nil pack")
	}
}

func TestCanonicalTools(t *testing.T) {
	tools := rulesystem.ListCanonicalTools()
	if len(tools) < 28 {
		t.Fatalf("canonical tools=%d want ~30", len(tools))
	}
	if _, ok := rulesystem.CanonicalByID("roll_dice"); !ok {
		t.Fatal("missing roll_dice")
	}
	if _, ok := rulesystem.CanonicalByID("advance_quest"); !ok {
		t.Fatal("missing advance_quest")
	}
}
