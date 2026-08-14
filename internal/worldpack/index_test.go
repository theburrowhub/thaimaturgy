package worldpack_test

import (
	"testing"

	"github.com/theburrowhub/thaimaturgy/internal/worldpack"
	_ "github.com/theburrowhub/thaimaturgy/internal/worldpack/profiles"
)

func TestBuildIndexes(t *testing.T) {
	p, err := worldpack.Builtin("dnd5e_shattered_vale")
	if err != nil {
		t.Fatal(err)
	}
	worldpack.BuildIndexes(p)

	if len(p.Indexes.ByCity["millhaven"]) < 6 {
		t.Fatalf("millhaven index=%d want >=6", len(p.Indexes.ByCity["millhaven"]))
	}
	if len(p.Indexes.ByRegion["whisperwood"]) == 0 {
		t.Fatal("expected whisperwood region index entries")
	}
	if len(p.Indexes.ByCreatureHabitat["forest"]) == 0 {
		t.Fatal("expected forest habitat creatures")
	}
}

func TestCreaturesInHabitat(t *testing.T) {
	p, err := worldpack.Builtin("dnd5e")
	if err != nil {
		t.Fatal(err)
	}
	worldpack.BuildIndexes(p)
	ids := p.CreaturesInHabitat("underground")
	if len(ids) == 0 {
		t.Fatal("expected underground creatures")
	}
}

func TestRollEncounterTable(t *testing.T) {
	p, err := worldpack.Builtin("dnd5e")
	if err != nil {
		t.Fatal(err)
	}
	row, table, err := p.RollEncounterTable("encounter_whisperwood", 5)
	if err != nil {
		t.Fatal(err)
	}
	if table.ID != "encounter_whisperwood" {
		t.Fatalf("table=%q", table.ID)
	}
	if row.Result == "" {
		t.Fatal("empty result")
	}
}

func TestNPCsAtLocation(t *testing.T) {
	p, err := worldpack.Builtin("dnd5e")
	if err != nil {
		t.Fatal(err)
	}
	npcs := p.NPCsAtLocation("the_gilded_anchor")
	if len(npcs) == 0 {
		t.Fatal("expected Tomas at Gilded Anchor")
	}
}

func TestNearbyLocations(t *testing.T) {
	p, err := worldpack.Builtin("dnd5e")
	if err != nil {
		t.Fatal(err)
	}
	near := p.NearbyLocations("millhaven_market")
	if len(near) == 0 {
		t.Fatal("expected connections from market")
	}
}
