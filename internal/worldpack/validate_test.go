package worldpack_test

import (
	"testing"

	"github.com/theburrowhub/thaimaturgy/internal/worldpack"
	_ "github.com/theburrowhub/thaimaturgy/internal/worldpack/worlds"
)

func TestValidateShatteredVale(t *testing.T) {
	p, err := worldpack.Builtin("dnd5e_shattered_vale")
	if err != nil {
		t.Fatal(err)
	}
	worldpack.BuildIndexes(p)
	if err := worldpack.ValidatePackStrict(p); err != nil {
		t.Fatal(err)
	}
}

func TestValidateRejectsEmptyPack(t *testing.T) {
	issues := worldpack.ValidatePack(&worldpack.Pack{})
	if len(issues) == 0 {
		t.Fatal("expected validation issues for empty pack")
	}
}

func TestValidateUnknownTool(t *testing.T) {
	p, err := worldpack.Builtin("dnd5e")
	if err != nil {
		t.Fatal(err)
	}
	p.Tools = append(p.Tools, worldpack.ToolBinding{CanonicalID: "not_a_real_tool", Enabled: true, Name: "bad"})
	issues := worldpack.ValidatePack(p)
	found := false
	for _, iss := range issues {
		if iss.Path != "" && iss.Message != "" {
			found = true
		}
	}
	if !found {
		t.Fatal("expected tool validation issue")
	}
}

func TestValidateStrictAggregates(t *testing.T) {
	err := worldpack.ValidatePackStrict(&worldpack.Pack{APIVersion: worldpack.APIVersion})
	if err == nil {
		t.Fatal("expected error")
	}
}
