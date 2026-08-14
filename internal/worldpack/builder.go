package worldpack

import (
	"encoding/json"
	"strings"

	"github.com/theburrowhub/thaimaturgy/internal/domain"
	"github.com/theburrowhub/thaimaturgy/internal/srd"
)

// NewBaseWorld creates a worldpack/v1 skeleton with common defaults.
func NewBaseWorld(id, name string, meta WorldMeta) *Pack {
	return &Pack{
		APIVersion: APIVersion,
		ID:         id,
		Name:       name,
		Version:    "1.0.0",
		Language:   "en",
		Setting: Setting{
			Name:                meta.SettingName,
			Summary:             meta.Summary,
			SuggestedRulesystem: meta.SuggestedRulesystem,
			PlayableWith:        meta.PlayableWith,
		},
		Tools: DefaultToolBindings(),
		OracleGuide: OracleGuide{
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
		Compatibility: EngineCompat{
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
func SetSettingTone(p *Pack, era, tone, summary string, tags ...string) {
	p.Setting.Era = era
	p.Setting.Tone = tone
	p.Setting.Summary = summary
	p.Setting.Tags = tags
}

// AddRegion appends a region.
func AddRegion(p *Pack, id, name, desc, biome string, tags ...string) {
	p.Regions = append(p.Regions, Region{
		ID: id, Name: name, Description: desc, Biome: biome, Tags: tags,
	})
}

// AddCity appends a city with optional districts.
func AddCity(p *Pack, id, name, regionID, desc string, districts []District, tags ...string) {
	p.Cities = append(p.Cities, City{
		ID: id, Name: name, RegionID: regionID, Description: desc, Districts: districts, Tags: tags,
	})
}

// AddDistrict is a convenience constructor for districts.
func AddDistrict(id, name, desc string, locationIDs []string, tags ...string) District {
	return District{
		ID: id, Name: name, Description: desc, LocationIDs: locationIDs, Tags: tags,
	}
}

// AddLocation appends a location.
func AddLocation(p *Pack, loc Location) {
	p.Locations = append(p.Locations, loc)
}

// AddNPC appends an NPC.
func AddNPC(p *Pack, npc WorldNPC) {
	p.NPCs = append(p.NPCs, npc)
}

// AddCreatureFromSRD adds a bestiary entry from the embedded SRD lookup.
func AddCreatureFromSRD(p *Pack, id, srdName string, habitats []string, encounterNotes, lore string, tags ...string) {
	sb, ok := srd.Lookup(srdName)
	if !ok {
		sb = domain.StatBlock{CR: "?"}
	}
	entry := CreatureEntry{
		ID: id, Name: titleCase(srdName), SRDName: srdName, StatBlock: sb,
		StatBlocks: map[string]domain.StatBlock{"dnd5e": sb},
		Habitats: habitats, CR: sb.CR, Tags: tags,
		EncounterNotes: encounterNotes, ToolAdapter: "lookup_creature", Lore: lore,
	}
	NormalizeCreatureEntry(&entry)
	p.Creatures = append(p.Creatures, entry)
}

// AddItem appends an item.
func AddItem(p *Pack, item WorldItem) {
	p.Items = append(p.Items, item)
}

// AddFaction appends a faction.
func AddFaction(p *Pack, id, name, desc, goals string) {
	p.Factions = append(p.Factions, domain.Faction{ID: id, Name: name, Description: desc, Goals: goals})
}

// AddLore appends a lore entry.
func AddLore(p *Pack, id, title, content, regionID string, tags ...string) {
	p.Lore = append(p.Lore, LoreEntry{ID: id, Title: title, Content: content, RegionID: regionID, Tags: tags})
}

// AddMap appends a map reference.
func AddMap(p *Pack, id, name, kind, path, desc, scale string) {
	p.Maps = append(p.Maps, MapRef{ID: id, Name: name, Kind: kind, Path: path, Description: desc, Scale: scale})
}

// AddEncounterTable appends an encounter table.
func AddEncounterTable(p *Pack, t EncounterTable) {
	p.EncounterTables = append(p.EncounterTables, t)
}

// AddLocationContents appends location contents.
func AddLocationContents(p *Pack, lc LocationContents) {
	p.LocationContents = append(p.LocationContents, lc)
}

// BindToolFromCanonical creates and appends a tool binding.
func BindToolFromCanonical(p *Pack, canonicalID, name, description, category string, preconditions []string, examples []ToolExample) {
	ct, ok := CanonicalByID(canonicalID)
	params := map[string]any{}
	if ok {
		params = ct.Parameters
	}
	raw, _ := json.Marshal(params)
	p.Tools = append(p.Tools, ToolBinding{
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
func LinkCityToRegion(p *Pack, regionID, cityID string) {
	for i := range p.Regions {
		if p.Regions[i].ID == regionID {
			for _, existing := range p.Regions[i].CityIDs {
				if existing == cityID {
					return
				}
			}
			p.Regions[i].CityIDs = append(p.Regions[i].CityIDs, cityID)
			return
		}
	}
}

// LinkLocationToCity adds location to city and optional district.
func LinkLocationToCity(p *Pack, cityID, districtID, locationID string) {
	for i := range p.Cities {
		if p.Cities[i].ID != cityID {
			continue
		}
		if !stringSliceContains(p.Cities[i].LocationIDs, locationID) {
			p.Cities[i].LocationIDs = append(p.Cities[i].LocationIDs, locationID)
		}
		if districtID != "" {
			for j := range p.Cities[i].Districts {
				if p.Cities[i].Districts[j].ID == districtID {
					if !stringSliceContains(p.Cities[i].Districts[j].LocationIDs, locationID) {
						p.Cities[i].Districts[j].LocationIDs = append(p.Cities[i].Districts[j].LocationIDs, locationID)
					}
				}
			}
		}
		return
	}
}

// LinkWildernessLocation adds a location ID to a region.
func LinkWildernessLocation(p *Pack, regionID, locationID string) {
	for i := range p.Regions {
		if p.Regions[i].ID == regionID {
			for _, existing := range p.Regions[i].LocationIDs {
				if existing == locationID {
					return
				}
			}
			p.Regions[i].LocationIDs = append(p.Regions[i].LocationIDs, locationID)
			return
		}
	}
}

func stringSliceContains(ss []string, want string) bool {
	for _, s := range ss {
		if s == want {
			return true
		}
	}
	return false
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


type WorldMeta struct {
	SettingName         string
	Summary             string
	SuggestedRulesystem string
	PlayableWith        []string
}

func SetWorldRules(p *Pack, magic, technology string) {
	p.Setting.WorldRules = WorldRules{Magic: magic, Technology: technology}
}

func SetWorldRulesFull(p *Pack, wr WorldRules) { p.Setting.WorldRules = wr }

func SetPolitics(p *Pack, summary string, majorPowers, conflicts []string) {
	p.Setting.Politics = Politics{Summary: summary, MajorPowers: majorPowers, Conflicts: conflicts}
}

func SetPoliticsFull(p *Pack, pol Politics) { p.Setting.Politics = pol }
