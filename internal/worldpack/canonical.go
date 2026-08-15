package worldpack

import "encoding/json"

// CanonicalTool describes a portable world-query tool that packs may bind to.
type CanonicalTool struct {
	ID          string
	Label       string
	Category    string
	Description string
	Parameters  map[string]any
}

var canonicalTools = []CanonicalTool{
	{
		ID: "get_region", Label: "Get Region", Category: "geography",
		Description: "Fetch a region by ID with cities and wilderness locations.",
		Parameters: map[string]any{"region_id": "string"},
	},
	{
		ID: "get_city", Label: "Get City", Category: "geography",
		Description: "Fetch a city by ID with districts and location list.",
		Parameters: map[string]any{"city_id": "string"},
	},
	{
		ID: "get_district", Label: "Get District", Category: "geography",
		Description: "Fetch a district within a city.",
		Parameters: map[string]any{"city_id": "string", "district_id": "string"},
	},
	{
		ID: "get_location", Label: "Get Location", Category: "geography",
		Description: "Fetch a location with description, tags, and connections.",
		Parameters: map[string]any{"location_id": "string"},
	},
	{
		ID: "list_city_locations", Label: "List City Locations", Category: "geography",
		Description: "List all locations in a city, optionally filtered by district.",
		Parameters: map[string]any{"city_id": "string", "district_id": "string (optional)"},
	},
	{
		ID: "list_location_npcs", Label: "List Location NPCs", Category: "population",
		Description: "List NPCs present at a location (from location contents and default locations).",
		Parameters: map[string]any{"location_id": "string"},
	},
	{
		ID: "list_location_creatures", Label: "List Location Creatures", Category: "encounters",
		Description: "List weighted creatures associated with a location.",
		Parameters: map[string]any{"location_id": "string"},
	},
	{
		ID: "list_location_items", Label: "List Location Items", Category: "treasure",
		Description: "List items found or sold at a location.",
		Parameters: map[string]any{"location_id": "string"},
	},
	{
		ID: "get_npc", Label: "Get NPC", Category: "population",
		Description: "Fetch an NPC with roleplay notes, stat block, and default location.",
		Parameters: map[string]any{"npc_id": "string"},
	},
	{
		ID: "get_creature", Label: "Get Creature", Category: "bestiary",
		Description: "Fetch a bestiary entry with full stat block and habitat notes.",
		Parameters: map[string]any{"creature_id": "string"},
	},
	{
		ID: "get_item", Label: "Get Item", Category: "treasure",
		Description: "Fetch an item with description and mechanics.",
		Parameters: map[string]any{"item_id": "string"},
	},
	{
		ID: "search_world", Label: "Search World", Category: "reference",
		Description: "Full-text search across locations, NPCs, items, and lore.",
		Parameters: map[string]any{"query": "string", "kinds": "array (optional)", "limit": "number (optional)"},
	},
	{
		ID: "roll_encounter_table", Label: "Roll Encounter Table", Category: "encounters",
		Description: "Roll on an encounter table by ID or context (biome, district, depth).",
		Parameters: map[string]any{"table_id": "string (optional)", "context": "string (optional)", "roll": "number (optional)"},
	},
	{
		ID: "find_nearby_locations", Label: "Find Nearby Locations", Category: "geography",
		Description: "List locations connected to or near a given location.",
		Parameters: map[string]any{"location_id": "string", "max_hops": "number (optional)"},
	},
	{
		ID: "get_faction", Label: "Get Faction", Category: "politics",
		Description: "Fetch a faction with goals and member NPC hints.",
		Parameters: map[string]any{"faction_id": "string"},
	},
	{
		ID: "get_lore", Label: "Get Lore", Category: "reference",
		Description: "Fetch a lore entry by ID or title.",
		Parameters: map[string]any{"lore_id": "string"},
	},
	{
		ID: "list_bestiary", Label: "List Bestiary", Category: "bestiary",
		Description: "List all creatures in the pack, optionally filtered by CR or tag.",
		Parameters: map[string]any{"cr_max": "string (optional)", "tag": "string (optional)"},
	},
	{
		ID: "filter_creatures_by_habitat", Label: "Filter Creatures By Habitat", Category: "bestiary",
		Description: "Return creature IDs matching a habitat (forest, urban, dungeon, coast, etc.).",
		Parameters: map[string]any{"habitat": "string"},
	},
	{
		ID: "get_map", Label: "Get Map", Category: "geography",
		Description: "Fetch map metadata for a region, city, or location.",
		Parameters: map[string]any{"map_id": "string"},
	},
	{
		ID: "describe_travel", Label: "Describe Travel", Category: "geography",
		Description: "Summarize travel between two regions or cities with hazards and duration.",
		Parameters: map[string]any{"from_id": "string", "to_id": "string", "mode": "string (optional)"},
	},
}

var canonicalByID map[string]CanonicalTool

func init() {
	canonicalByID = make(map[string]CanonicalTool, len(canonicalTools))
	for _, t := range canonicalTools {
		canonicalByID[t.ID] = t
	}
}

// CanonicalByID returns a canonical tool definition by ID.
func CanonicalByID(id string) (CanonicalTool, bool) {
	t, ok := canonicalByID[id]
	return t, ok
}

// ListCanonicalTools returns all canonical world query tools.
func ListCanonicalTools() []CanonicalTool {
	out := make([]CanonicalTool, len(canonicalTools))
	copy(out, canonicalTools)
	return out
}

// bindTool creates a ToolBinding from a canonical tool ID.
func bindTool(canonicalID, name, description, category string, preconditions []string, examples []ToolExample) ToolBinding {
	params := map[string]any{}
	if ct, ok := canonicalByID[canonicalID]; ok {
		params = ct.Parameters
	}
	raw, _ := json.Marshal(params)
	return ToolBinding{
		CanonicalID:   canonicalID,
		Enabled:       true,
		Name:          name,
		Description:   description,
		Parameters:    raw,
		Category:      category,
		Preconditions: preconditions,
		Examples:      examples,
	}
}

// DefaultToolBindings returns enabled bindings for all canonical world tools.
func DefaultToolBindings() []ToolBinding {
	out := make([]ToolBinding, 0, len(canonicalTools))
	for _, ct := range canonicalTools {
		out = append(out, bindTool(ct.ID, ct.Label, ct.Description, ct.Category, nil, nil))
	}
	return out
}
