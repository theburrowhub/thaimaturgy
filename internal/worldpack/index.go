package worldpack

import "fmt"

// BuildIndexes rebuilds all lookup indexes on a pack in place.
func BuildIndexes(p *Pack) {
	if p == nil {
		return
	}
	idx := Indexes{
		ByCity:               map[string][]string{},
		ByRegion:             map[string][]string{},
		ByTag:                map[string][]string{},
		ByCreatureHabitat:    map[string][]string{},
		ByDistrict:           map[string][]string{},
		ByFaction:            map[string][]string{},
		NPCLocationIndex:     map[string]string{},
		LocationContentIndex: map[string]string{},
	}

	locationIDs := map[string]struct{}{}
	for _, loc := range p.Locations {
		locationIDs[loc.ID] = struct{}{}
		indexAppend(idx.ByCity, loc.CityID, loc.ID)
		indexAppend(idx.ByRegion, loc.RegionID, loc.ID)
		indexAppend(idx.ByDistrict, loc.DistrictID, loc.ID)
		for _, tag := range loc.Tags {
			indexAppend(idx.ByTag, tag, loc.ID)
		}
	}

	for _, city := range p.Cities {
		for _, locID := range city.LocationIDs {
			indexAppend(idx.ByCity, city.ID, locID)
		}
		indexAppend(idx.ByRegion, city.RegionID, city.ID)
		for _, tag := range city.Tags {
			indexAppend(idx.ByTag, tag, city.ID)
		}
		for _, d := range city.Districts {
			for _, locID := range d.LocationIDs {
				indexAppend(idx.ByDistrict, d.ID, locID)
				indexAppend(idx.ByCity, city.ID, locID)
			}
		}
	}

	for _, reg := range p.Regions {
		for _, locID := range reg.LocationIDs {
			indexAppend(idx.ByRegion, reg.ID, locID)
		}
		for _, cityID := range reg.CityIDs {
			indexAppend(idx.ByRegion, reg.ID, cityID)
		}
		for _, tag := range reg.Tags {
			indexAppend(idx.ByTag, tag, reg.ID)
		}
	}

	for _, c := range p.Creatures {
		for _, h := range c.Habitats {
			indexAppend(idx.ByCreatureHabitat, h, c.ID)
		}
		for _, tag := range c.Tags {
			indexAppend(idx.ByTag, tag, c.ID)
		}
	}

	for _, n := range p.NPCs {
		if n.DefaultLocation != "" {
			idx.NPCLocationIndex[n.ID] = n.DefaultLocation
		}
		indexAppend(idx.ByFaction, n.FactionID, n.ID)
		for _, tag := range n.Tags {
			indexAppend(idx.ByTag, tag, n.ID)
		}
	}

	for _, it := range p.Items {
		for _, locID := range it.LocationIDs {
			indexAppend(idx.ByCity, locID, it.ID) // also indexed by location via tag
			indexAppend(idx.ByTag, "item:"+locID, it.ID)
		}
		for _, tag := range it.Tags {
			indexAppend(idx.ByTag, tag, it.ID)
		}
	}

	for i, lc := range p.LocationContents {
		idx.LocationContentIndex[lc.LocationID] = lc.LocationID
		_ = i
	}

	// Deduplicate index slices.
	for k, v := range idx.ByCity {
		idx.ByCity[k] = uniqueStrings(v)
	}
	for k, v := range idx.ByRegion {
		idx.ByRegion[k] = uniqueStrings(v)
	}
	for k, v := range idx.ByTag {
		idx.ByTag[k] = uniqueStrings(v)
	}
	for k, v := range idx.ByCreatureHabitat {
		idx.ByCreatureHabitat[k] = uniqueStrings(v)
	}
	for k, v := range idx.ByDistrict {
		idx.ByDistrict[k] = uniqueStrings(v)
	}
	for k, v := range idx.ByFaction {
		idx.ByFaction[k] = uniqueStrings(v)
	}

	p.Indexes = idx
}

// LocationsInCity returns location IDs for a city from the index.
func (p *Pack) LocationsInCity(cityID string) []string {
	if p == nil {
		return nil
	}
	return append([]string(nil), p.Indexes.ByCity[cityID]...)
}

// CreaturesInHabitat returns creature IDs for a habitat from the index.
func (p *Pack) CreaturesInHabitat(habitat string) []string {
	if p == nil {
		return nil
	}
	return append([]string(nil), p.Indexes.ByCreatureHabitat[habitat]...)
}

// SearchWorld performs a simple text search across pack entities.
func (p *Pack) SearchWorld(query string, kinds []string, limit int) []SearchHit {
	if p == nil || query == "" {
		return nil
	}
	kindSet := map[string]struct{}{}
	allKinds := len(kinds) == 0
	for _, k := range kinds {
		kindSet[k] = struct{}{}
	}
	var hits []SearchHit
	add := func(kind, id, label, snippet string) {
		if !allKinds {
			if _, ok := kindSet[kind]; !ok {
				return
			}
		}
		if containsFold(label, query) || containsFold(snippet, query) {
			hits = append(hits, SearchHit{Kind: kind, ID: id, Label: label, Snippet: snippet})
		}
	}
	for _, loc := range p.Locations {
		add("location", loc.ID, loc.Name, loc.Description)
	}
	for _, n := range p.NPCs {
		add("npc", n.ID, n.Name, n.Personality+" "+n.Role)
	}
	for _, it := range p.Items {
		add("item", it.ID, it.Name, it.Description)
	}
	for _, l := range p.Lore {
		add("lore", l.ID, l.Title, l.Content)
	}
	for _, c := range p.Cities {
		add("city", c.ID, c.Name, c.Description)
	}
	for _, r := range p.Regions {
		add("region", r.ID, r.Name, r.Description)
	}
	if limit > 0 && len(hits) > limit {
		hits = hits[:limit]
	}
	return hits
}

// SearchHit is one result from SearchWorld.
type SearchHit struct {
	Kind    string `json:"kind"`
	ID      string `json:"id"`
	Label   string `json:"label"`
	Snippet string `json:"snippet,omitempty"`
}

// RollEncounterTable resolves a row from an encounter table by roll value.
func (p *Pack) RollEncounterTable(tableID string, roll int) (*EncounterTableRow, *EncounterTable, error) {
	t := p.EncounterTable(tableID)
	if t == nil {
		return nil, nil, fmt.Errorf("unknown encounter table %q", tableID)
	}
	for i := range t.Rows {
		if parseRollRange(t.Rows[i].Roll, roll) {
			return &t.Rows[i], t, nil
		}
	}
	return nil, t, fmt.Errorf("no row matches roll %d on table %q", roll, tableID)
}

// NearbyLocations returns directly connected locations.
func (p *Pack) NearbyLocations(locationID string) []string {
	loc := p.Location(locationID)
	if loc == nil {
		return nil
	}
	return append([]string(nil), loc.Connections...)
}

// NPCsAtLocation returns NPC IDs present at a location.
func (p *Pack) NPCsAtLocation(locationID string) []string {
	var out []string
	if lc := p.LocationContentsFor(locationID); lc != nil {
		out = append(out, lc.NPCIDs...)
	}
	for _, n := range p.NPCs {
		if n.DefaultLocation == locationID {
			out = append(out, n.ID)
		}
	}
	return uniqueStrings(out)
}

// ItemsAtLocation returns item IDs at a location.
func (p *Pack) ItemsAtLocation(locationID string) []string {
	var out []string
	if lc := p.LocationContentsFor(locationID); lc != nil {
		out = append(out, lc.ItemIDs...)
	}
	for _, it := range p.Items {
		for _, lid := range it.LocationIDs {
			if lid == locationID {
				out = append(out, it.ID)
			}
		}
	}
	return uniqueStrings(out)
}

// CreaturesAtLocation returns weighted creatures at a location.
func (p *Pack) CreaturesAtLocation(locationID string) []WeightedCreature {
	if lc := p.LocationContentsFor(locationID); lc != nil {
		return append([]WeightedCreature(nil), lc.CreatureWeights...)
	}
	return nil
}

// TravelDescription summarizes travel between two region or city IDs.
func (p *Pack) TravelDescription(fromID, toID string) string {
	fromReg := p.Region(fromID)
	toReg := p.Region(toID)
	if fromReg != nil && toReg != nil {
		return fromReg.TravelNotes + " Route to " + toReg.Name + ": " + toReg.Description
	}
	fromCity := p.City(fromID)
	toCity := p.City(toID)
	if fromCity != nil && toCity != nil {
		return "Travel from " + fromCity.Name + " to " + toCity.Name + " via trade roads; allow 2-4 days on foot."
	}
	return "Unknown route between " + fromID + " and " + toID
}
