package domain

// topology.go provides the zone-adjacency graph queries the DM uses to keep the
// party's marching order: which zones are directly reachable and in which
// direction, and how to get from one zone to another (so "go back to the
// entrance hall" can be resolved even when it is several zones away).

// ZoneNeighbors returns the directional exits of a zone (its outgoing edges in
// the adjacency graph). Nil when the zone is unknown or has no exits.
func (a *Adventure) ZoneNeighbors(zoneID string) []ZoneExit {
	z := a.Zone(zoneID)
	if z == nil {
		return nil
	}
	return z.Exits
}

// AdjacentZones returns the ids of the zones directly reachable from zoneID.
// Locked passages are included unless includeLocked is false.
func (a *Adventure) AdjacentZones(zoneID string, includeLocked bool) []string {
	var out []string
	seen := map[string]bool{}
	for _, e := range a.ZoneNeighbors(zoneID) {
		if e.Locked && !includeLocked {
			continue
		}
		if e.To == "" || seen[e.To] {
			continue
		}
		seen[e.To] = true
		out = append(out, e.To)
	}
	return out
}

// PathStep is one hop along a zone path: leave in Direction to reach zone To.
type PathStep struct {
	Direction Direction `json:"direction,omitempty"`
	To        string    `json:"to"`
	Locked    bool      `json:"locked,omitempty"`
}

// PathZones finds a shortest sequence of hops from zone `from` to zone `to`
// over the directional graph (breadth-first, so fewest zones traversed). Locked
// passages are only used when includeLocked is true. Returns (steps, true) on
// success — an empty slice when from == to — or (nil, false) when either zone is
// unknown or no route exists.
func (a *Adventure) PathZones(from, to string, includeLocked bool) ([]PathStep, bool) {
	if a.Zone(from) == nil || a.Zone(to) == nil {
		return nil, false
	}
	if from == to {
		return []PathStep{}, true
	}
	// BFS, remembering the edge used to reach each zone so we can rebuild the path.
	type edgeTo struct {
		prev string
		step PathStep
	}
	came := map[string]edgeTo{from: {}}
	queue := []string{from}
	for len(queue) > 0 {
		cur := queue[0]
		queue = queue[1:]
		for _, e := range a.ZoneNeighbors(cur) {
			if e.To == "" || (e.Locked && !includeLocked) {
				continue
			}
			if _, ok := came[e.To]; ok {
				continue
			}
			came[e.To] = edgeTo{prev: cur, step: PathStep{Direction: e.Direction, To: e.To, Locked: e.Locked}}
			if e.To == to {
				// Reconstruct from `to` back to `from`.
				var rev []PathStep
				for at := to; at != from; {
					et := came[at]
					rev = append(rev, et.step)
					at = et.prev
				}
				// Reverse into forward order.
				steps := make([]PathStep, len(rev))
				for i := range rev {
					steps[len(rev)-1-i] = rev[i]
				}
				return steps, true
			}
			queue = append(queue, e.To)
		}
	}
	return nil, false
}

// ReachableZones returns every zone id reachable from `from` (excluding `from`
// itself), in breadth-first order. Locked passages are only traversed when
// includeLocked is true.
func (a *Adventure) ReachableZones(from string, includeLocked bool) []string {
	if a.Zone(from) == nil {
		return nil
	}
	seen := map[string]bool{from: true}
	var out []string
	queue := []string{from}
	for len(queue) > 0 {
		cur := queue[0]
		queue = queue[1:]
		for _, e := range a.ZoneNeighbors(cur) {
			if e.To == "" || (e.Locked && !includeLocked) || seen[e.To] {
				continue
			}
			seen[e.To] = true
			out = append(out, e.To)
			queue = append(queue, e.To)
		}
	}
	return out
}
