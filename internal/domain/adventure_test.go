package domain

import "testing"

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
