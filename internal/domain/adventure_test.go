package domain

import (
	"strings"
	"testing"

	"github.com/theburrowhub/thaimaturgy/internal/rules"
)

func validAdv() *Adventure {
	return &Adventure{
		SchemaVersion: SchemaVersion,
		ID:            "adv",
		Title:         "Adventure",
		Zones: []Zone{{
			ID:   "z1",
			Name: "Zone",
			Rooms: []Room{{
				ID:       "r1",
				Name:     "Room",
				NPCIDs:   []string{"n1"},
				EventIDs: []string{"e1"},
				Exits:    []Exit{{To: "r2"}},
			}, {
				ID:   "r2",
				Name: "Room 2",
			}},
		}},
		NPCs:   []NPC{{ID: "n1", Name: "NPC One", DefaultLocation: "r1"}},
		Events: []Event{{ID: "e1", Name: "Event One"}},
	}
}

func TestValidateAdventureOK(t *testing.T) {
	if errs := ValidateAdventure(validAdv(), nil); len(errs) != 0 {
		t.Errorf("expected valid adventure, got %v", errs)
	}
}

func TestValidateAdventureRequiredFields(t *testing.T) {
	a := &Adventure{}
	errs := ValidateAdventure(a, nil)
	if len(errs) == 0 {
		t.Fatal("expected errors for empty adventure")
	}
}

func TestAdventureRulesRequirementIsExplicitAndLegacyMappingIsClosed(t *testing.T) {
	explicit := validAdv()
	explicit.Ruleset = &rules.Requirement{ID: "fatecore", Version: "0.1.0"}
	explicit.System = "A misleading display label"
	got, ok := explicit.RulesRequirement()
	if !ok || got != *explicit.Ruleset {
		t.Fatalf("explicit requirement = %#v, %v", got, ok)
	}

	known := &Adventure{System: "La llamada de Cthulhu"}
	got, ok = known.RulesRequirement()
	if !ok || got.ID != "coc7e" || got.Version != "0.1.0" {
		t.Fatalf("legacy requirement = %#v, %v", got, ok)
	}
	known.Migrate()
	if known.Ruleset == nil || *known.Ruleset != got {
		t.Fatalf("migration did not persist requirement: %#v", known.Ruleset)
	}

	unknown := &Adventure{System: "Homebrew Mystery"}
	if got, ok := unknown.RulesRequirement(); ok {
		t.Fatalf("unknown system was guessed as %#v", got)
	}
	unknown.Migrate()
	if unknown.Ruleset != nil {
		t.Fatalf("unknown system was migrated as %#v", unknown.Ruleset)
	}

	invalidExplicit := &Adventure{
		System:  "D&D 5e",
		Ruleset: &rules.Requirement{ID: "Bad ID", Version: "0.1.0"},
	}
	if got, ok := invalidExplicit.RulesRequirement(); ok {
		t.Fatalf("invalid explicit requirement fell back to legacy label as %#v", got)
	}
}

func TestValidateAdventureRejectsInvalidRulesRequirement(t *testing.T) {
	adventure := validAdv()
	adventure.Ruleset = &rules.Requirement{ID: "Bad ID", Version: "latest"}
	errs := ValidateAdventure(adventure, nil)
	if len(errs) == 0 || !strings.Contains(errs[0].Error(), "invalid ruleset requirement") {
		t.Fatalf("validation errors = %v", errs)
	}
}

func TestValidateAdventureBadRefs(t *testing.T) {
	a := validAdv()
	a.Zones[0].Rooms[0].NPCIDs = []string{"ghost"}
	a.Zones[0].Rooms[0].EventIDs = []string{"nope"}
	a.Zones[0].Rooms[0].Exits = []Exit{{To: "void"}}
	errs := ValidateAdventure(a, nil)
	if len(errs) < 3 {
		t.Errorf("expected at least 3 referential errors, got %d: %v", len(errs), errs)
	}
}

func TestValidateAdventureImagePresence(t *testing.T) {
	a := validAdv()
	a.Zones[0].MapImage = "assets/map.png"
	present := map[string]bool{"assets/map.png": true}
	if errs := ValidateAdventure(a, func(p string) bool { return present[p] }); len(errs) != 0 {
		t.Errorf("expected no errors, got %v", errs)
	}
	if errs := ValidateAdventure(a, func(p string) bool { return false }); len(errs) == 0 {
		t.Error("expected an error for missing image")
	}
}

func TestAdventureLookups(t *testing.T) {
	a := validAdv()
	if a.Zone("z1") == nil {
		t.Error("Zone lookup failed")
	}
	if r, z := a.Room("r1"); r == nil || z == nil || z.ID != "z1" {
		t.Error("Room lookup failed")
	}
	if a.NPC("n1") == nil {
		t.Error("NPC lookup failed")
	}
	if a.Event("e1") == nil {
		t.Error("Event lookup failed")
	}
}

func TestImageIDReferencesValidation(t *testing.T) {
	a := validAdv()
	a.Images = []ImageRef{{ID: "map1", Path: "assets/map1.png", Kind: "map"}, {ID: "art1", Path: "assets/art1.png", Kind: "art"}}
	a.Zones[0].ImageIDs = []string{"map1"}
	a.Zones[0].Rooms[0].ImageIDs = []string{"art1", "ghost-img"} // ghost-img is dangling
	a.NPCs[0].ImageIDs = []string{"art1"}

	errs := ValidateAdventure(a, nil)
	if len(errs) != 1 {
		t.Fatalf("expected exactly 1 error (dangling image ref), got %d: %v", len(errs), errs)
	}
	if !strings.Contains(errs[0].Error(), "ghost-img") {
		t.Errorf("error should name the dangling image, got %v", errs[0])
	}
}

func TestImageResolvers(t *testing.T) {
	a := validAdv()
	a.Images = []ImageRef{
		{ID: "scene", Path: "assets/scene.png", Kind: "art"},
		{ID: "themap", Path: "assets/themap.png", Kind: "map"},
	}
	// Zone with two image_ids and no direct map_image: ZoneMap prefers the map.
	a.Zones[0].ImageIDs = []string{"scene", "themap"}
	if got := a.ZoneMap(&a.Zones[0]); got != "assets/themap.png" {
		t.Errorf("ZoneMap = %q, want assets/themap.png (map kind preferred)", got)
	}
	// Room combines a direct path with a catalog id, deduped and in order.
	r := &a.Zones[0].Rooms[0]
	r.Image = "assets/direct.png"
	r.ImageIDs = []string{"scene", "themap"}
	imgs := a.RoomImages(r)
	want := []string{"assets/direct.png", "assets/scene.png", "assets/themap.png"}
	if len(imgs) != 3 || imgs[0] != want[0] || imgs[1] != want[1] || imgs[2] != want[2] {
		t.Errorf("RoomImages = %v, want %v", imgs, want)
	}
	// Unknown ids resolve to nothing.
	if got := a.resolveImages("", []string{"nope"}); len(got) != 0 {
		t.Errorf("unknown id should resolve to nothing, got %v", got)
	}
}

func TestImageRefsDedup(t *testing.T) {
	a := validAdv()
	a.Zones[0].MapImage = "assets/map.png"
	a.NPCs[0].Image = "assets/n1.png"
	a.Images = []ImageRef{{ID: "m", Path: "assets/map.png"}} // duplicate of zone map
	refs := a.ImageRefs()
	seen := map[string]int{}
	for _, r := range refs {
		seen[r]++
	}
	if seen["assets/map.png"] != 1 {
		t.Errorf("expected map.png once, got %d", seen["assets/map.png"])
	}
	if seen["assets/n1.png"] != 1 {
		t.Error("expected n1.png present")
	}
}
