package rulesystem_test

import (
	"testing"

	"github.com/theburrowhub/thaimaturgy/internal/rulesystem"
	_ "github.com/theburrowhub/thaimaturgy/internal/rulesystem/profiles"
)

func TestAnalyzeExcerptCombat(t *testing.T) {
	ex := rulesystem.AnalyzeExcerpt("When you make an attack roll, compare the result to the target armor class. On a hit, roll damage dice.")
	if ex.Category != "combat" {
		t.Fatalf("category=%q want combat", ex.Category)
	}
	if ex.Confidence < 0.3 {
		t.Fatalf("confidence too low: %v", ex.Confidence)
	}
	if len(ex.Keywords) == 0 {
		t.Fatal("expected keywords")
	}
}

func TestAnalyzeExcerptMagic(t *testing.T) {
	ex := rulesystem.AnalyzeExcerpt("To cast a spell, expend a spell slot and provide verbal and somatic components. Concentration may apply.")
	if ex.Category != "magic" {
		t.Fatalf("category=%q want magic", ex.Category)
	}
}

func TestAnalyzeExcerptEmpty(t *testing.T) {
	ex := rulesystem.AnalyzeExcerpt("")
	if ex.Text != "" || ex.Category != "" {
		t.Fatalf("unexpected: %+v", ex)
	}
}

func TestAnalyzeExcerpts(t *testing.T) {
	out := rulesystem.AnalyzeExcerpts([]string{
		"Skill checks use d20 plus modifier against a DC.",
		"Character creation begins with ability scores.",
	})
	if len(out) != 2 {
		t.Fatalf("got %d", len(out))
	}
}

func TestMergePDFExcerpts(t *testing.T) {
	p, _ := rulesystem.Builtin("dnd5e")
	excerpts := rulesystem.AnalyzeExcerpts([]string{
		"Combat: Attack rolls use d20 against armor class.",
		"Magic: Spell slots power arcane casting.",
	})
	merged, err := rulesystem.MergePDFExcerpts(p, excerpts)
	if err != nil {
		t.Fatal(err)
	}
	if len(merged.RawExcerpts) != 2 {
		t.Fatalf("raw excerpts=%d", len(merged.RawExcerpts))
	}
	foundCombat := false
	for _, ch := range merged.Chapters {
		if ch.ID == "combat" && len(ch.Sections) > 0 {
			foundCombat = true
		}
	}
	if !foundCombat {
		t.Fatal("expected combat chapter with imported sections")
	}
}
