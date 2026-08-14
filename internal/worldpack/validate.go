package worldpack

import (
	"fmt"
	"strings"
)

// ValidationIssue describes a single validation problem.
type ValidationIssue struct {
	Path    string
	Message string
}

func (v ValidationIssue) Error() string {
	if v.Path == "" {
		return v.Message
	}
	return fmt.Sprintf("%s: %s", v.Path, v.Message)
}

// ValidatePack performs deep validation on a world pack definition.
func ValidatePack(p *Pack) []ValidationIssue {
	if p == nil {
		return []ValidationIssue{{Message: "pack is nil"}}
	}
	var issues []ValidationIssue
	add := func(path, msg string) {
		issues = append(issues, ValidationIssue{Path: path, Message: msg})
	}

	if p.ID == "" {
		add("id", "required")
	}
	if p.Name == "" {
		add("name", "required")
	}
	if p.APIVersion != "" && p.APIVersion != APIVersion {
		add("api_version", fmt.Sprintf("expected %q", APIVersion))
	}
	if strings.TrimSpace(p.Setting.Name) == "" {
		add("setting.name", "required")
	}
	if strings.TrimSpace(p.Setting.WorldRules.Magic) == "" {
		add("setting.world_rules.magic", "required")
	}
	if strings.TrimSpace(p.Setting.Politics.Summary) == "" {
		add("setting.politics.summary", "required")
	}
	if len(p.Regions) == 0 {
		add("regions", "at least one region required")
	}
	if len(p.Cities) == 0 {
		add("cities", "at least one city required")
	}
	if len(p.Locations) == 0 {
		add("locations", "at least one location required")
	}

	regionIDs := map[string]struct{}{}
	cityIDs := map[string]struct{}{}
	districtIDs := map[string]struct{}{}
	locationIDs := map[string]struct{}{}
	npcIDs := map[string]struct{}{}
	creatureIDs := map[string]struct{}{}
	itemIDs := map[string]struct{}{}
	factionIDs := map[string]struct{}{}
	loreIDs := map[string]struct{}{}
	mapIDs := map[string]struct{}{}
	tableIDs := map[string]struct{}{}

	for _, r := range p.Regions {
		if r.ID == "" {
			add("regions", "region missing id")
			continue
		}
		if _, dup := regionIDs[r.ID]; dup {
			add("regions."+r.ID, "duplicate id")
		}
		regionIDs[r.ID] = struct{}{}
	}
	for _, c := range p.Cities {
		if c.ID == "" {
			add("cities", "city missing id")
			continue
		}
		if _, dup := cityIDs[c.ID]; dup {
			add("cities."+c.ID, "duplicate id")
		}
		cityIDs[c.ID] = struct{}{}
		if c.RegionID != "" {
			if _, ok := regionIDs[c.RegionID]; !ok {
				add("cities."+c.ID, fmt.Sprintf("unknown region %q", c.RegionID))
			}
		}
		for _, d := range c.Districts {
			if d.ID == "" {
				add("cities."+c.ID+".districts", "district missing id")
				continue
			}
			key := c.ID + "/" + d.ID
			if _, dup := districtIDs[key]; dup {
				add("cities."+c.ID+".districts."+d.ID, "duplicate id")
			}
			districtIDs[key] = struct{}{}
		}
	}
	for _, loc := range p.Locations {
		if loc.ID == "" {
			add("locations", "location missing id")
			continue
		}
		if _, dup := locationIDs[loc.ID]; dup {
			add("locations."+loc.ID, "duplicate id")
		}
		locationIDs[loc.ID] = struct{}{}
		if loc.CityID != "" {
			if _, ok := cityIDs[loc.CityID]; !ok {
				add("locations."+loc.ID, fmt.Sprintf("unknown city %q", loc.CityID))
			}
		}
		if loc.RegionID != "" {
			if _, ok := regionIDs[loc.RegionID]; !ok {
				add("locations."+loc.ID, fmt.Sprintf("unknown region %q", loc.RegionID))
			}
		}
		if loc.DistrictID != "" && loc.CityID != "" {
			key := loc.CityID + "/" + loc.DistrictID
			if _, ok := districtIDs[key]; !ok {
				add("locations."+loc.ID, fmt.Sprintf("unknown district %q in city %q", loc.DistrictID, loc.CityID))
			}
		}
		for _, conn := range loc.Connections {
			if conn != "" {
				if _, ok := locationIDs[conn]; !ok {
					// may reference not-yet-seen; second pass below
				}
			}
		}
	}
	for _, loc := range p.Locations {
		for _, conn := range loc.Connections {
			if conn != "" {
				if _, ok := locationIDs[conn]; !ok {
					add("locations."+loc.ID, fmt.Sprintf("connection references unknown location %q", conn))
				}
			}
		}
	}
	for _, n := range p.NPCs {
		if n.ID == "" {
			add("npcs", "npc missing id")
			continue
		}
		if _, dup := npcIDs[n.ID]; dup {
			add("npcs."+n.ID, "duplicate id")
		}
		npcIDs[n.ID] = struct{}{}
		if n.DefaultLocation != "" {
			if _, ok := locationIDs[n.DefaultLocation]; !ok {
				add("npcs."+n.ID, fmt.Sprintf("default_location references unknown location %q", n.DefaultLocation))
			}
		}
		if n.FactionID != "" {
			// validated below after factions collected
		}
	}
	for _, c := range p.Creatures {
		if c.ID == "" {
			add("creatures", "creature missing id")
			continue
		}
		if _, dup := creatureIDs[c.ID]; dup {
			add("creatures."+c.ID, "duplicate id")
		}
		creatureIDs[c.ID] = struct{}{}
	}
	for _, it := range p.Items {
		if it.ID == "" {
			add("items", "item missing id")
			continue
		}
		if _, dup := itemIDs[it.ID]; dup {
			add("items."+it.ID, "duplicate id")
		}
		itemIDs[it.ID] = struct{}{}
		for _, lid := range it.LocationIDs {
			if _, ok := locationIDs[lid]; !ok {
				add("items."+it.ID, fmt.Sprintf("location reference unknown %q", lid))
			}
		}
	}
	for _, f := range p.Factions {
		if f.ID == "" {
			add("factions", "faction missing id")
			continue
		}
		factionIDs[f.ID] = struct{}{}
	}
	for _, n := range p.NPCs {
		if n.FactionID != "" {
			if _, ok := factionIDs[n.FactionID]; !ok {
				add("npcs."+n.ID, fmt.Sprintf("unknown faction %q", n.FactionID))
			}
		}
	}
	for _, l := range p.Lore {
		if l.ID == "" {
			add("lore", "lore entry missing id")
			continue
		}
		loreIDs[l.ID] = struct{}{}
		if l.RegionID != "" {
			if _, ok := regionIDs[l.RegionID]; !ok {
				add("lore."+l.ID, fmt.Sprintf("unknown region %q", l.RegionID))
			}
		}
	}
	for _, m := range p.Maps {
		if m.ID == "" {
			add("maps", "map missing id")
			continue
		}
		mapIDs[m.ID] = struct{}{}
	}
	for _, t := range p.EncounterTables {
		if t.ID == "" {
			add("encounter_tables", "table missing id")
			continue
		}
		if _, dup := tableIDs[t.ID]; dup {
			add("encounter_tables."+t.ID, "duplicate id")
		}
		tableIDs[t.ID] = struct{}{}
		if len(t.Rows) == 0 {
			add("encounter_tables."+t.ID, "at least one row required")
		}
		for _, row := range t.Rows {
			for _, cid := range row.CreatureIDs {
				if cid != "" {
					if _, ok := creatureIDs[cid]; !ok {
						add("encounter_tables."+t.ID, fmt.Sprintf("row references unknown creature %q", cid))
					}
				}
			}
		}
	}
	for _, lc := range p.LocationContents {
		if lc.LocationID == "" {
			add("location_contents", "missing location_id")
			continue
		}
		if _, ok := locationIDs[lc.LocationID]; !ok {
			add("location_contents."+lc.LocationID, "unknown location")
		}
		for _, nid := range lc.NPCIDs {
			if _, ok := npcIDs[nid]; !ok {
				add("location_contents."+lc.LocationID, fmt.Sprintf("unknown npc %q", nid))
			}
		}
		for _, iid := range lc.ItemIDs {
			if _, ok := itemIDs[iid]; !ok {
				add("location_contents."+lc.LocationID, fmt.Sprintf("unknown item %q", iid))
			}
		}
		for _, wc := range lc.CreatureWeights {
			if _, ok := creatureIDs[wc.CreatureID]; !ok {
				add("location_contents."+lc.LocationID, fmt.Sprintf("unknown creature %q", wc.CreatureID))
			}
		}
		if lc.EncounterTableID != "" {
			if _, ok := tableIDs[lc.EncounterTableID]; !ok {
				add("location_contents."+lc.LocationID, fmt.Sprintf("unknown encounter table %q", lc.EncounterTableID))
			}
		}
	}
	for i, tb := range p.Tools {
		if _, ok := canonicalByID[tb.CanonicalID]; !ok {
			add(fmt.Sprintf("tools[%d].canonical_id", i), fmt.Sprintf("unknown canonical tool %q", tb.CanonicalID))
		}
	}
	return issues
}

// ValidatePackStrict returns an error if any issues are found.
func ValidatePackStrict(p *Pack) error {
	issues := ValidatePack(p)
	if len(issues) == 0 {
		return nil
	}
	msgs := make([]string, len(issues))
	for i, iss := range issues {
		msgs[i] = iss.Error()
	}
	return fmt.Errorf("pack validation failed:\n  - %s", strings.Join(msgs, "\n  - "))
}
