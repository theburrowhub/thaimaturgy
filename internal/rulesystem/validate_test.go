package rulesystem_test

import (
	"strings"
	"testing"

	"github.com/theburrowhub/thaimaturgy/internal/rulesystem"
	_ "github.com/theburrowhub/thaimaturgy/internal/rulesystem/profiles"
)

func TestValidatePackDnD5e(t *testing.T) {
	p, err := rulesystem.Builtin("dnd5e")
	if err != nil {
		t.Fatal(err)
	}
	issues := rulesystem.ValidatePack(p)
	if len(issues) > 0 {
		t.Fatalf("validation issues: %v", issues)
	}
}

func TestValidatePackStrict(t *testing.T) {
	p, _ := rulesystem.Builtin("d100")
	if err := rulesystem.ValidatePackStrict(p); err != nil {
		t.Fatal(err)
	}
}

func TestValidatePackNil(t *testing.T) {
	issues := rulesystem.ValidatePack(nil)
	if len(issues) == 0 {
		t.Fatal("expected issues for nil pack")
	}
}

func TestValidatePackMissingID(t *testing.T) {
	p := &rulesystem.Pack{Chapters: []rulesystem.RuleChapter{{ID: "x", Title: "T", Sections: []rulesystem.Section{{ID: "s", Title: "S", Body: "b"}}}}}
	issues := rulesystem.ValidatePack(p)
	found := false
	for _, iss := range issues {
		if iss.Path == "id" {
			found = true
		}
	}
	if !found {
		t.Fatal("expected missing id issue")
	}
}

func TestValidateInvalidFormula(t *testing.T) {
	p, _ := rulesystem.Builtin("dnd5e")
	p.Formulas = append(p.Formulas, rulesystem.FormulaDef{ID: "bad", Label: "Bad", Expression: "1 + * 2"})
	issues := rulesystem.ValidatePack(p)
	found := false
	for _, iss := range issues {
		if iss.Path == "formulas.bad" {
			found = true
		}
	}
	if !found {
		t.Fatal("expected formula validation issue")
	}
}

func TestValidateUnknownTool(t *testing.T) {
	p, _ := rulesystem.Builtin("dnd5e")
	p.Tools = append(p.Tools, rulesystem.ToolBinding{CanonicalID: "nonexistent_tool", Enabled: true, Name: "Bad"})
	issues := rulesystem.ValidatePack(p)
	found := false
	for _, iss := range issues {
		if strings.Contains(iss.Message, "nonexistent_tool") {
			found = true
		}
	}
	if !found {
		t.Fatal("expected unknown tool issue")
	}
}
