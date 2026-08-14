package worldpack_test

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/theburrowhub/thaimaturgy/internal/srd"
	"github.com/theburrowhub/thaimaturgy/internal/worldpack"
	_ "github.com/theburrowhub/thaimaturgy/internal/worldpack/profiles"
)

func TestBuiltinIDs(t *testing.T) {
	ids := worldpack.BuiltinIDs()
	if len(ids) < 1 {
		t.Fatalf("expected at least 1 builtin, got %d", len(ids))
	}
}

func TestBuiltinShatteredVale(t *testing.T) {
	p, err := worldpack.Builtin("dnd5e_shattered_vale")
	if err != nil {
		t.Fatal(err)
	}
	if p.ID != "dnd5e_shattered_vale" {
		t.Fatalf("id=%q", p.ID)
	}
	if p.Setting.RulesystemID != "dnd5e" {
		t.Fatalf("rulesystem=%q", p.Setting.RulesystemID)
	}
	if len(p.Regions) < 4 {
		t.Fatalf("regions=%d want >=4", len(p.Regions))
	}
	if len(p.Cities) < 3 {
		t.Fatalf("cities=%d want >=3", len(p.Cities))
	}
	if len(p.Locations) < 20 {
		t.Fatalf("locations=%d want >=20", len(p.Locations))
	}
	if len(p.NPCs) < 15 {
		t.Fatalf("npcs=%d want >=15", len(p.NPCs))
	}
	if len(p.Creatures) < 15 {
		t.Fatalf("creatures=%d want >=15 (all SRD creatures)", len(p.Creatures))
	}
	if len(p.Items) < 25 {
		t.Fatalf("items=%d want >=25", len(p.Items))
	}
	if len(p.EncounterTables) < 8 {
		t.Fatalf("encounter_tables=%d want >=8", len(p.EncounterTables))
	}
	if len(p.Tools) < 20 {
		t.Fatalf("tools=%d want >=20", len(p.Tools))
	}
}

func TestAliasDnD5e(t *testing.T) {
	p, err := worldpack.Builtin("dnd5e")
	if err != nil {
		t.Fatal(err)
	}
	if p.Setting.Name != "The Shattered Vale" {
		t.Fatalf("setting=%q", p.Setting.Name)
	}
}

func TestGenerateJSONSize(t *testing.T) {
	p, err := worldpack.Generate(worldpack.GenerateOptions{TemplateID: "dnd5e"})
	if err != nil {
		t.Fatal(err)
	}
	data, err := json.Marshal(p)
	if err != nil {
		t.Fatal(err)
	}
	if len(data) < 1000 {
		t.Fatalf("json size=%d want >=1000 lines equivalent content", len(data))
	}
	t.Logf("generated JSON bytes: %d", len(data))
}

func TestAllSRDCreaturesPresent(t *testing.T) {
	p, err := worldpack.Builtin("dnd5e_shattered_vale")
	if err != nil {
		t.Fatal(err)
	}
	names := map[string]bool{}
	for _, c := range p.Creatures {
		if c.SRDName != "" {
			names[c.SRDName] = true
		}
	}
	for _, want := range srd.Names() {
		if !names[want] {
			t.Errorf("missing SRD creature %q", want)
		}
	}
}

func TestMillhavenLocations(t *testing.T) {
	p, err := worldpack.Builtin("dnd5e")
	if err != nil {
		t.Fatal(err)
	}
	locs := p.LocationsInCity("millhaven")
	if len(locs) < 6 {
		t.Fatalf("millhaven locations=%d want >=6", len(locs))
	}
}

func TestSearchWorld(t *testing.T) {
	p, err := worldpack.Builtin("dnd5e")
	if err != nil {
		t.Fatal(err)
	}
	worldpack.BuildIndexes(p)
	hits := p.SearchWorld("Red Hand", nil, 10)
	if len(hits) == 0 {
		t.Fatal("expected search hits for Red Hand")
	}
	found := false
	for _, h := range hits {
		if strings.Contains(h.Label, "Cassian") || strings.Contains(h.Snippet, "Red Hand") {
			found = true
		}
	}
	if !found {
		t.Fatalf("hits=%v", hits)
	}
}
