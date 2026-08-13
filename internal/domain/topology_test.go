package domain

import "testing"

// graph:  entrance --east--> hall --north--> library
//
//	hall --down(locked)--> crypt
func topoAdventure() *Adventure {
	return &Adventure{
		ID: "m", Title: "M",
		Zones: []Zone{
			{ID: "entrance", Rooms: []Room{{ID: "e1"}}, Exits: []ZoneExit{{Direction: DirEast, To: "hall"}}},
			{ID: "hall", Rooms: []Room{{ID: "h1"}}, Exits: []ZoneExit{
				{Direction: DirWest, To: "entrance"},
				{Direction: DirNorth, To: "library"},
				{Direction: DirDown, To: "crypt", Locked: true},
			}},
			{ID: "library", Rooms: []Room{{ID: "l1"}}, Exits: []ZoneExit{{Direction: DirSouth, To: "hall"}}},
			{ID: "crypt", Rooms: []Room{{ID: "c1"}}, Exits: []ZoneExit{{Direction: DirUp, To: "hall"}}},
		},
	}
}

func TestAdjacentZones(t *testing.T) {
	a := topoAdventure()
	adj := a.AdjacentZones("hall", false) // exclude locked
	if len(adj) != 2 {
		t.Fatalf("adjacent (no locked) = %v; want 2 (entrance, library)", adj)
	}
	adjL := a.AdjacentZones("hall", true) // include locked
	if len(adjL) != 3 {
		t.Fatalf("adjacent (with locked) = %v; want 3", adjL)
	}
}

func TestPathZones(t *testing.T) {
	a := topoAdventure()

	if steps, ok := a.PathZones("entrance", "entrance", false); !ok || len(steps) != 0 {
		t.Errorf("path to self = %v,%v; want [],true", steps, ok)
	}

	steps, ok := a.PathZones("entrance", "library", false)
	if !ok || len(steps) != 2 || steps[0].To != "hall" || steps[1].To != "library" {
		t.Fatalf("entrance->library = %+v,%v", steps, ok)
	}
	if steps[0].Direction != DirEast || steps[1].Direction != DirNorth {
		t.Errorf("path directions = %q,%q; want east,north", steps[0].Direction, steps[1].Direction)
	}

	// crypt only reachable through a locked passage.
	if _, ok := a.PathZones("entrance", "crypt", false); ok {
		t.Errorf("expected no path to crypt without locked passages")
	}
	if steps, ok := a.PathZones("entrance", "crypt", true); !ok || steps[len(steps)-1].To != "crypt" {
		t.Errorf("path to crypt (locked allowed) = %+v,%v", steps, ok)
	}

	// unknown zones.
	if _, ok := a.PathZones("entrance", "ghost", true); ok {
		t.Errorf("expected no path to unknown zone")
	}
}

func TestReachableZones(t *testing.T) {
	a := topoAdventure()
	r := a.ReachableZones("entrance", false)
	if len(r) != 2 { // hall, library (crypt is behind a locked door)
		t.Fatalf("reachable (no locked) from entrance = %v; want 2", r)
	}
	rL := a.ReachableZones("entrance", true)
	if len(rL) != 3 {
		t.Fatalf("reachable (with locked) = %v; want 3", rL)
	}
}
