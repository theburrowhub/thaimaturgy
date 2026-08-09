package domain

import "testing"

func TestLocationReflectsSetLocationUnderLock(t *testing.T) {
	adv := &Adventure{ID: "m", Title: "M", Zones: []Zone{
		{ID: "z1", Rooms: []Room{{ID: "r1"}}},
		{ID: "z2", Rooms: []Room{{ID: "r2"}}},
	}}
	st := NewSessionState("s", adv)
	st.SetLocation("z2", "r2", "Room 2")
	if z, r := st.Location(); z != "z2" || r != "r2" {
		t.Fatalf("Location = %q,%q; want z2,r2", z, r)
	}
}

func TestZoneMapPrefersDirectThenMapCatalog(t *testing.T) {
	adv := &Adventure{ID: "m", Title: "M",
		Images: []ImageRef{
			{ID: "art1", Path: "assets/art/x.png", Kind: "art"},
			{ID: "map1", Path: "assets/maps/z.png", Kind: "map"},
		},
		Zones: []Zone{{ID: "z", MapImage: "assets/maps/direct.png", ImageIDs: []string{"art1", "map1"}}},
	}
	if got := adv.ZoneMap(&adv.Zones[0]); got != "assets/maps/direct.png" {
		t.Errorf("ZoneMap (direct) = %q; want the map_image", got)
	}
	adv.Zones[0].MapImage = ""
	if got := adv.ZoneMap(&adv.Zones[0]); got != "assets/maps/z.png" {
		t.Errorf("ZoneMap (catalog) = %q; want the kind:map image", got)
	}
}
