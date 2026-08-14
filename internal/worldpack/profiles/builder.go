package profiles

import (
	"encoding/json"
	"strings"

	"github.com/theburrowhub/thaimaturgy/internal/domain"
	"github.com/theburrowhub/thaimaturgy/internal/srd"
	"github.com/theburrowhub/thaimaturgy/internal/worldpack"
)

// NewBaseWorld creates a worldpack/v1 skeleton with common defaults.
func NewBaseWorld(id, name, settingName, rulesystemID string) *worldpack.Pack {
	return &worldpack.Pack{
		APIVersion: worldpack.APIVersion,
		ID:         id,
		Name:       name,
		Version:    "1.0.0",
		Language:   "en",
		Setting: worldpack.Setting{
			Name:         settingName,
			RulesystemID: rulesystemID,
		},
		Tools: worldpack.DefaultToolBindings(),
		OracleGuide: worldpack.OracleGuide{
			Principles: []string{
				"Query the world pack for established locations, NPCs, and encounters before inventing.",
				"Invent only when the pack has no matching content or the party goes off the authored map.",
				"Use encounter tables for random wilderness and urban night scenes.",
			},
			ToolPriority: []string{
				"get_location", "list_city_locations", "get_npc", "roll_encounter_table", "search_world",
			},
			AntiPatterns: []string{
				"Inventing a named shopkeeper when list_location_npcs returns candidates.",
				"Rolling custom random encounters when a biome table exists.",
			},
		},
		Compatibility: worldpack.EngineCompat{
			RoomType:     "domain.Room",
			NPCType:      "domain.NPC",
			CreatureType: "domain.StatBlock",
			ToolMap: map[string]string{
				"get_location":     "get_room",
				"get_npc":          "get_npc",
				"get_creature":     "lookup_creature",
				"search_world":     "search_module",
			},
			Notes: "Future engine wiring maps worldpack query tools to adventure module lookups.",
		},
		Metadata: map[string]string{"generator": "worldpack/v1"},
	}
}

// SetSettingTone assigns era and tone on the pack setting.
func SetSettingTone(p *worldpack.Pack, era, tone, summary string, tags ...string) {
	p.Setting.Era = era
	p.Setting.Tone = tone
	p.Setting.Summary = summary
	p.Setting.Tags = tags
}

// AddRegion appends a region.
func AddRegion(p *worldpack.Pack, id, name, desc, biome string, tags ...string) {
	p.Regions = append(p.Regions, worldpack.Region{
		ID: id, Name: name, Description: desc, Biome: biome, Tags: tags,
	})
}

// AddCity appends a city with optional districts.
func AddCity(p *worldpack.Pack, id, name, regionID, desc string, districts []worldpack.District, tags ...string) {
	p.Cities = append(p.Cities, worldpack.City{
		ID: id, Name: name, RegionID: regionID, Description: desc, Districts: districts, Tags: tags,
	})
}

// AddDistrict is a convenience constructor for districts.
func AddDistrict(id, name, desc string, locationIDs []string, tags ...string) worldpack.District {
	return worldpack.District{
		ID: id, Name: name, Description: desc, LocationIDs: locationIDs, Tags: tags,
	}
}

// AddLocation appends a location.
func AddLocation(p *worldpack.Pack, loc worldpack.Location) {
	p.Locations = append(p.Locations, loc)
}

// AddNPC appends an NPC.
func AddNPC(p *worldpack.Pack, npc worldpack.WorldNPC) {
	p.NPCs = append(p.NPCs, npc)
}

// AddCreatureFromSRD adds a bestiary entry from the embedded SRD lookup.
func AddCreatureFromSRD(p *worldpack.Pack, id, srdName string, habitats []string, encounterNotes, lore string, tags ...string) {
	sb, ok := srd.Lookup(srdName)
	if !ok {
		sb = domain.StatBlock{CR: "?"}
	}
	p.Creatures = append(p.Creatures, worldpack.CreatureEntry{
		ID:             id,
		Name:           titleCase(srdName),
		SRDName:        srdName,
		StatBlock:      sb,
		Habitats:       habitats,
		CR:             sb.CR,
		Tags:           tags,
		EncounterNotes: encounterNotes,
		ToolAdapter:    "lookup_creature",
		Lore:           lore,
	})
}

// AddItem appends an item.
func AddItem(p *worldpack.Pack, item worldpack.WorldItem) {
	p.Items = append(p.Items, item)
}

// AddFaction appends a faction.
func AddFaction(p *worldpack.Pack, id, name, desc, goals string) {
	p.Factions = append(p.Factions, domain.Faction{ID: id, Name: name, Description: desc, Goals: goals})
}

// AddLore appends a lore entry.
func AddLore(p *worldpack.Pack, id, title, content, regionID string, tags ...string) {
	p.Lore = append(p.Lore, worldpack.LoreEntry{ID: id, Title: title, Content: content, RegionID: regionID, Tags: tags})
}

// AddMap appends a map reference.
func AddMap(p *worldpack.Pack, id, name, kind, path, desc, scale string) {
	p.Maps = append(p.Maps, worldpack.MapRef{ID: id, Name: name, Kind: kind, Path: path, Description: desc, Scale: scale})
}

// AddEncounterTable appends an encounter table.
func AddEncounterTable(p *worldpack.Pack, t worldpack.EncounterTable) {
	p.EncounterTables = append(p.EncounterTables, t)
}

// AddLocationContents appends location contents.
func AddLocationContents(p *worldpack.Pack, lc worldpack.LocationContents) {
	p.LocationContents = append(p.LocationContents, lc)
}

// BindToolFromCanonical creates and appends a tool binding.
func BindToolFromCanonical(p *worldpack.Pack, canonicalID, name, description, category string, preconditions []string, examples []worldpack.ToolExample) {
	ct, ok := worldpack.CanonicalByID(canonicalID)
	params := map[string]any{}
	if ok {
		params = ct.Parameters
	}
	raw, _ := json.Marshal(params)
	p.Tools = append(p.Tools, worldpack.ToolBinding{
		CanonicalID:   canonicalID,
		Enabled:       true,
		Name:          name,
		Description:   description,
		Parameters:    raw,
		Category:      category,
		Preconditions: preconditions,
		Examples:      examples,
	})
	if p.Compatibility.ToolMap == nil {
		p.Compatibility.ToolMap = map[string]string{}
	}
	p.Compatibility.ToolMap[name] = canonicalID
}

// LinkCityToRegion adds city ID to region's city list.
func LinkCityToRegion(p *worldpack.Pack, regionID, cityID string) {
	for i := range p.Regions {
		if p.Regions[i].ID == regionID {
			p.Regions[i].CityIDs = append(p.Regions[i].CityIDs, cityID)
			return
		}
	}
}

// LinkLocationToCity adds location to city and optional district.
func LinkLocationToCity(p *worldpack.Pack, cityID, districtID, locationID string) {
	for i := range p.Cities {
		if p.Cities[i].ID != cityID {
			continue
		}
		p.Cities[i].LocationIDs = append(p.Cities[i].LocationIDs, locationID)
		if districtID != "" {
			for j := range p.Cities[i].Districts {
				if p.Cities[i].Districts[j].ID == districtID {
					p.Cities[i].Districts[j].LocationIDs = append(p.Cities[i].Districts[j].LocationIDs, locationID)
				}
			}
		}
		return
	}
}

// LinkWildernessLocation adds a location ID to a region.
func LinkWildernessLocation(p *worldpack.Pack, regionID, locationID string) {
	for i := range p.Regions {
		if p.Regions[i].ID == regionID {
			p.Regions[i].LocationIDs = append(p.Regions[i].LocationIDs, locationID)
			return
		}
	}
}

func titleCase(s string) string {
	if s == "" {
		return s
	}
	parts := splitWords(s)
	for i, p := range parts {
		if len(p) > 0 {
			parts[i] = strings.ToUpper(p[:1]) + p[1:]
		}
	}
	return strings.Join(parts, " ")
}

func splitWords(s string) []string {
	return strings.Fields(strings.ReplaceAll(s, "_", " "))
}
