package domain

import "testing"

func TestNormalizeDirection(t *testing.T) {
	cases := map[string]Direction{
		"N": DirNorth, "north": DirNorth, "Norte": DirNorth,
		"sur": DirSouth, "S": DirSouth,
		"oeste": DirWest, "o": DirWest, "W": DirWest,
		"este": DirEast, "e": DirEast,
		"ne": DirNortheast, "so": DirSouthwest,
		"up": DirUp, "abajo": DirDown, "dentro": DirIn, "fuera": DirOut,
	}
	for in, want := range cases {
		if got, ok := NormalizeDirection(in); !ok || got != want {
			t.Errorf("NormalizeDirection(%q) = %q,%v; want %q,true", in, got, ok, want)
		}
	}
	for _, bad := range []string{"", "   ", "sideways", "xyz"} {
		if got, ok := NormalizeDirection(bad); ok {
			t.Errorf("NormalizeDirection(%q) = %q,true; want _,false", bad, got)
		}
	}
}

func TestDirectionOpposite(t *testing.T) {
	pairs := [][2]Direction{
		{DirNorth, DirSouth}, {DirEast, DirWest}, {DirUp, DirDown},
		{DirIn, DirOut}, {DirNortheast, DirSouthwest}, {DirNorthwest, DirSoutheast},
	}
	for _, p := range pairs {
		if p[0].Opposite() != p[1] || p[1].Opposite() != p[0] {
			t.Errorf("opposite mismatch for %v/%v", p[0], p[1])
		}
	}
}

func TestMigrateBackfillsAndNormalizes(t *testing.T) {
	adv := &Adventure{
		ID: "m", Title: "M",
		Zones: []Zone{
			{ID: "a", Connections: []string{"b"}, Rooms: []Room{
				{ID: "a1", Exits: []Exit{{To: "b1", Direction: "Norte"}}},
			}},
			{ID: "b", Exits: []ZoneExit{{To: "a", Direction: "sur"}}, Rooms: []Room{{ID: "b1"}}},
		},
	}
	adv.Migrate()

	// Legacy Connections backfilled into Exits (direction unknown).
	if len(adv.Zones[0].Exits) != 1 || adv.Zones[0].Exits[0].To != "b" {
		t.Fatalf("zone a exits after migrate = %+v", adv.Zones[0].Exits)
	}
	if adv.Zones[0].Exits[0].Direction != "" {
		t.Errorf("migrated legacy exit should have empty direction, got %q", adv.Zones[0].Exits[0].Direction)
	}
	// Explicit zone-exit direction normalized (sur -> south).
	if adv.Zones[1].Exits[0].Direction != DirSouth {
		t.Errorf("zone b exit direction = %q; want south", adv.Zones[1].Exits[0].Direction)
	}
	// Room exit direction normalized (Norte -> north).
	if adv.Zones[0].Rooms[0].Exits[0].Direction != "north" {
		t.Errorf("room exit direction = %q; want north", adv.Zones[0].Rooms[0].Exits[0].Direction)
	}
	// Idempotent.
	before := len(adv.Zones[0].Exits)
	adv.Migrate()
	if len(adv.Zones[0].Exits) != before {
		t.Errorf("Migrate not idempotent: exits grew to %d", len(adv.Zones[0].Exits))
	}
}

func TestStartRoomID(t *testing.T) {
	adv := &Adventure{Zones: []Zone{{ID: "z", Rooms: []Room{{ID: "first"}, {ID: "second"}}}}}
	if got := adv.StartRoomID(); got != "first" {
		t.Errorf("fallback StartRoomID = %q; want first", got)
	}
	adv.StartRoom = "second"
	if got := adv.StartRoomID(); got != "second" {
		t.Errorf("explicit StartRoomID = %q; want second", got)
	}
	adv.StartRoom = "nope"
	if got := adv.StartRoomID(); got != "first" {
		t.Errorf("invalid StartRoom should fall back to first, got %q", got)
	}
}

func TestValidateZoneExitsAndStartRoom(t *testing.T) {
	adv := &Adventure{
		ID: "m", Title: "M", StartRoom: "ghost",
		Zones: []Zone{
			{ID: "a", Rooms: []Room{{ID: "a1"}}, Exits: []ZoneExit{
				{To: "b", Direction: DirEast},         // ok
				{To: "nope", Direction: DirWest},      // unknown zone
				{To: "b", Direction: Direction("xy")}, // invalid direction
			}},
			{ID: "b", Rooms: []Room{{ID: "b1"}}},
		},
	}
	errs := ValidateAdventure(adv, nil)
	joined := ""
	for _, e := range errs {
		joined += e.Error() + "\n"
	}
	for _, want := range []string{"unknown zone \"nope\"", "invalid direction", "start_room references unknown room \"ghost\""} {
		if !contains(joined, want) {
			t.Errorf("validation missing %q; got:\n%s", want, joined)
		}
	}
}

func contains(s, sub string) bool {
	return len(sub) == 0 || (len(s) >= len(sub) && indexOf(s, sub) >= 0)
}

func indexOf(s, sub string) int {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return i
		}
	}
	return -1
}
