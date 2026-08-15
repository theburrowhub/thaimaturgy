// Package worldpack defines portable world content catalogs for thAImaturgy.
// Packs describe cities, regions, NPCs, creatures, items, and encounter tables
// so a session can query "what's in this city?" without inventing content.
// Packs are intentionally isolated from the running oracle/engine.
package worldpack

import (
	"encoding/json"

	"github.com/theburrowhub/thaimaturgy/internal/domain"
)

const APIVersion = "worldpack/v1"

// Pack is the on-disk definition of a game world for thAImaturgy.
type Pack struct {
	APIVersion string `json:"api_version" yaml:"api_version"`
	ID         string `json:"id" yaml:"id"`
	Name       string `json:"name" yaml:"name"`
	Version    string `json:"version,omitempty" yaml:"version,omitempty"`
	Language   string `json:"language,omitempty" yaml:"language,omitempty"`

	Setting Setting `json:"setting" yaml:"setting"`

	Regions         []Region          `json:"regions" yaml:"regions"`
	Cities          []City            `json:"cities" yaml:"cities"`
	Locations       []Location        `json:"locations" yaml:"locations"`
	NPCs            []WorldNPC        `json:"npcs" yaml:"npcs"`
	Creatures       []CreatureEntry   `json:"creatures" yaml:"creatures"`
	Items           []WorldItem       `json:"items" yaml:"items"`
	Factions        []domain.Faction  `json:"factions" yaml:"factions"`
	Lore            []LoreEntry       `json:"lore" yaml:"lore"`
	Maps            []MapRef          `json:"maps" yaml:"maps"`
	EncounterTables []EncounterTable  `json:"encounter_tables" yaml:"encounter_tables"`
	LocationContents []LocationContents `json:"location_contents" yaml:"location_contents"`

	Indexes Indexes `json:"indexes" yaml:"indexes"`

	Tools         []ToolBinding `json:"tools" yaml:"tools"`
	OracleGuide   OracleGuide   `json:"oracle_guide" yaml:"oracle_guide"`
	Compatibility EngineCompat  `json:"compatibility" yaml:"compatibility"`

	Metadata map[string]string `json:"metadata,omitempty" yaml:"metadata,omitempty"`
}

// Setting is everything that defines THIS world (ambience, rules of reality, politics).
// SuggestedRulesystem is optional — you can run Caribdus with Savage Worlds or D&D.
type Setting struct {
	Name                string     `json:"name" yaml:"name"`
	Era                 string     `json:"era,omitempty" yaml:"era,omitempty"`
	Tone                string     `json:"tone,omitempty" yaml:"tone,omitempty"`
	Summary             string     `json:"summary,omitempty" yaml:"summary,omitempty"`
	Tags                []string   `json:"tags,omitempty" yaml:"tags,omitempty"`
	WorldRules          WorldRules `json:"world_rules" yaml:"world_rules"`
	Politics            Politics   `json:"politics" yaml:"politics"`
	SuggestedRulesystem string     `json:"suggested_rulesystem,omitempty" yaml:"suggested_rulesystem,omitempty"`
	PlayableWith        []string   `json:"playable_with,omitempty" yaml:"playable_with,omitempty"`
}

type WorldRules struct {
	Magic             string `json:"magic" yaml:"magic"`
	MagicNotes        string `json:"magic_notes,omitempty" yaml:"magic_notes,omitempty"`
	Technology        string `json:"technology,omitempty" yaml:"technology,omitempty"`
	TechnologyNotes   string `json:"technology_notes,omitempty" yaml:"technology_notes,omitempty"`
	DeathAndAfterlife string `json:"death_and_afterlife,omitempty" yaml:"death_and_afterlife,omitempty"`
	Travel            string `json:"travel,omitempty" yaml:"travel,omitempty"`
}

type Politics struct {
	Summary     string   `json:"summary" yaml:"summary"`
	Government  string   `json:"government,omitempty" yaml:"government,omitempty"`
	MajorPowers []string `json:"major_powers,omitempty" yaml:"major_powers,omitempty"`
	Conflicts   []string `json:"conflicts,omitempty" yaml:"conflicts,omitempty"`
	LawAndOrder string   `json:"law_and_order,omitempty" yaml:"law_and_order,omitempty"`
}

// Region is a large geographic area containing cities and wilderness.
type Region struct {
	ID          string   `json:"id" yaml:"id"`
	Name        string   `json:"name" yaml:"name"`
	Description string   `json:"description,omitempty" yaml:"description,omitempty"`
	Biome       string   `json:"biome,omitempty" yaml:"biome,omitempty"`
	CityIDs     []string `json:"city_ids,omitempty" yaml:"city_ids,omitempty"`
	LocationIDs []string `json:"location_ids,omitempty" yaml:"location_ids,omitempty"`
	Tags        []string `json:"tags,omitempty" yaml:"tags,omitempty"`
	MapID       string   `json:"map_id,omitempty" yaml:"map_id,omitempty"`
	TravelNotes string   `json:"travel_notes,omitempty" yaml:"travel_notes,omitempty"`
}

// City is a settlement with districts and linked locations.
type City struct {
	ID          string     `json:"id" yaml:"id"`
	Name        string     `json:"name" yaml:"name"`
	RegionID    string     `json:"region_id" yaml:"region_id"`
	Description string     `json:"description,omitempty" yaml:"description,omitempty"`
	Population  string     `json:"population,omitempty" yaml:"population,omitempty"`
	Government  string     `json:"government,omitempty" yaml:"government,omitempty"`
	Districts   []District `json:"districts,omitempty" yaml:"districts,omitempty"`
	LocationIDs []string   `json:"location_ids,omitempty" yaml:"location_ids,omitempty"`
	Tags        []string   `json:"tags,omitempty" yaml:"tags,omitempty"`
	MapID       string     `json:"map_id,omitempty" yaml:"map_id,omitempty"`
}

// District is a neighborhood or quarter within a city.
type District struct {
	ID          string   `json:"id" yaml:"id"`
	Name        string   `json:"name" yaml:"name"`
	Description string   `json:"description,omitempty" yaml:"description,omitempty"`
	LocationIDs []string `json:"location_ids,omitempty" yaml:"location_ids,omitempty"`
	Tags        []string `json:"tags,omitempty" yaml:"tags,omitempty"`
	EncounterTableID string `json:"encounter_table_id,omitempty" yaml:"encounter_table_id,omitempty"`
}

// Location is a place within a city or wilderness (shop, dungeon entrance, grove).
type Location struct {
	ID          string   `json:"id" yaml:"id"`
	Name        string   `json:"name" yaml:"name"`
	Kind        string   `json:"kind,omitempty" yaml:"kind,omitempty"` // tavern, temple, wilderness, dungeon
	CityID      string   `json:"city_id,omitempty" yaml:"city_id,omitempty"`
	DistrictID  string   `json:"district_id,omitempty" yaml:"district_id,omitempty"`
	RegionID    string   `json:"region_id,omitempty" yaml:"region_id,omitempty"`
	Description string   `json:"description,omitempty" yaml:"description,omitempty"`
	ReadAloud   string   `json:"read_aloud,omitempty" yaml:"read_aloud,omitempty"`
	DMNotes     string   `json:"dm_notes,omitempty" yaml:"dm_notes,omitempty"`
	Tags        []string `json:"tags,omitempty" yaml:"tags,omitempty"`
	MapID       string   `json:"map_id,omitempty" yaml:"map_id,omitempty"`
	Connections []string `json:"connections,omitempty" yaml:"connections,omitempty"` // adjacent location IDs
}

// WorldNPC is a non-player character with roleplay and optional combat stats.
type WorldNPC struct {
	ID              string              `json:"id" yaml:"id"`
	Name            string              `json:"name" yaml:"name"`
	Role            string              `json:"role,omitempty" yaml:"role,omitempty"`
	Appearance      string              `json:"appearance,omitempty" yaml:"appearance,omitempty"`
	Personality     string              `json:"personality,omitempty" yaml:"personality,omitempty"`
	Motivations     string              `json:"motivations,omitempty" yaml:"motivations,omitempty"`
	Secrets         string              `json:"secrets,omitempty" yaml:"secrets,omitempty"`
	Voice           string              `json:"voice,omitempty" yaml:"voice,omitempty"`
	Knowledge       []string            `json:"knowledge,omitempty" yaml:"knowledge,omitempty"`
	SampleDialogue  []string            `json:"sample_dialogue,omitempty" yaml:"sample_dialogue,omitempty"`
	Disposition     string              `json:"disposition,omitempty" yaml:"disposition,omitempty"`
	FactionID       string              `json:"faction_id,omitempty" yaml:"faction_id,omitempty"`
	StatBlock       *domain.StatBlock   `json:"stat_block,omitempty" yaml:"stat_block,omitempty"`
	DefaultLocation string              `json:"default_location,omitempty" yaml:"default_location,omitempty"`
	ToolBindings    []NPCToolBinding    `json:"tool_bindings,omitempty" yaml:"tool_bindings,omitempty"`
	Tags            []string            `json:"tags,omitempty" yaml:"tags,omitempty"`
}

// NPCToolBinding links an NPC to a canonical world tool with default parameters.
type NPCToolBinding struct {
	ToolID     string         `json:"tool_id" yaml:"tool_id"`
	Parameters map[string]any `json:"parameters,omitempty" yaml:"parameters,omitempty"`
	Notes      string         `json:"notes,omitempty" yaml:"notes,omitempty"`
}

// CreatureEntry is a bestiary entry with optional per-rulesystem stat blocks.
type CreatureEntry struct {
	ID             string            `json:"id" yaml:"id"`
	Name           string            `json:"name" yaml:"name"`
	SRDName        string            `json:"srd_name,omitempty" yaml:"srd_name,omitempty"`
	StatBlock      domain.StatBlock             `json:"stat_block,omitempty" yaml:"stat_block,omitempty"`
	StatBlocks     map[string]domain.StatBlock  `json:"stat_blocks,omitempty" yaml:"stat_blocks,omitempty"`
	Habitats       []string          `json:"habitats,omitempty" yaml:"habitats,omitempty"`
	CR             string            `json:"cr,omitempty" yaml:"cr,omitempty"`
	Tags           []string          `json:"tags,omitempty" yaml:"tags,omitempty"`
	EncounterNotes string            `json:"encounter_notes,omitempty" yaml:"encounter_notes,omitempty"`
	ToolAdapter    string            `json:"tool_adapter,omitempty" yaml:"tool_adapter,omitempty"`
	Lore           string            `json:"lore,omitempty" yaml:"lore,omitempty"`
}

// WorldItem is gear, treasure, or a magic item with optional mechanics text.
type WorldItem struct {
	ID          string   `json:"id" yaml:"id"`
	Name        string   `json:"name" yaml:"name"`
	Kind        string   `json:"kind,omitempty" yaml:"kind,omitempty"` // weapon, armor, potion, gear, magic
	Rarity      string   `json:"rarity,omitempty" yaml:"rarity,omitempty"`
	Description string   `json:"description,omitempty" yaml:"description,omitempty"`
	Mechanics   string   `json:"mechanics,omitempty" yaml:"mechanics,omitempty"`
	ValueGP     int      `json:"value_gp,omitempty" yaml:"value_gp,omitempty"`
	Tags        []string `json:"tags,omitempty" yaml:"tags,omitempty"`
	LocationIDs []string `json:"location_ids,omitempty" yaml:"location_ids,omitempty"`
}

// LoreEntry is world background the DM can reference.
type LoreEntry struct {
	ID      string   `json:"id" yaml:"id"`
	Title   string   `json:"title" yaml:"title"`
	Content string   `json:"content" yaml:"content"`
	Tags    []string `json:"tags,omitempty" yaml:"tags,omitempty"`
	RegionID string  `json:"region_id,omitempty" yaml:"region_id,omitempty"`
}

// MapRef catalogs a map asset or metadata reference.
type MapRef struct {
	ID          string `json:"id" yaml:"id"`
	Name        string `json:"name" yaml:"name"`
	Kind        string `json:"kind,omitempty" yaml:"kind,omitempty"` // regional, city, dungeon
	Path        string `json:"path,omitempty" yaml:"path,omitempty"`
	Description string `json:"description,omitempty" yaml:"description,omitempty"`
	Scale       string `json:"scale,omitempty" yaml:"scale,omitempty"`
}

// EncounterTable is a rollable random encounter table keyed by context.
type EncounterTable struct {
	ID          string              `json:"id" yaml:"id"`
	Name        string              `json:"name" yaml:"name"`
	Context     string              `json:"context,omitempty" yaml:"context,omitempty"` // forest, urban_night, dungeon
	Biome       string              `json:"biome,omitempty" yaml:"biome,omitempty"`
	DistrictID  string              `json:"district_id,omitempty" yaml:"district_id,omitempty"`
	DungeonDepth int                `json:"dungeon_depth,omitempty" yaml:"dungeon_depth,omitempty"`
	Dice        string              `json:"dice" yaml:"dice"`
	Description string              `json:"description,omitempty" yaml:"description,omitempty"`
	Rows        []EncounterTableRow `json:"rows" yaml:"rows"`
	Tags        []string            `json:"tags,omitempty" yaml:"tags,omitempty"`
}

// EncounterTableRow is one row of an encounter table.
type EncounterTableRow struct {
	Roll       string   `json:"roll" yaml:"roll"`
	Result     string   `json:"result" yaml:"result"`
	CreatureIDs []string `json:"creature_ids,omitempty" yaml:"creature_ids,omitempty"`
	Quantity   string   `json:"quantity,omitempty" yaml:"quantity,omitempty"`
	Notes      string   `json:"notes,omitempty" yaml:"notes,omitempty"`
}

// LocationContents describes what is found at a location.
type LocationContents struct {
	LocationID      string            `json:"location_id" yaml:"location_id"`
	NPCIDs          []string          `json:"npc_ids,omitempty" yaml:"npc_ids,omitempty"`
	ItemIDs         []string          `json:"item_ids,omitempty" yaml:"item_ids,omitempty"`
	CreatureWeights []WeightedCreature `json:"creature_weights,omitempty" yaml:"creature_weights,omitempty"`
	Features        []LocationFeature `json:"features,omitempty" yaml:"features,omitempty"`
	EncounterTableID string           `json:"encounter_table_id,omitempty" yaml:"encounter_table_id,omitempty"`
	Notes           string            `json:"notes,omitempty" yaml:"notes,omitempty"`
}

// WeightedCreature is a creature ID with relative encounter weight.
type WeightedCreature struct {
	CreatureID string  `json:"creature_id" yaml:"creature_id"`
	Weight     float64 `json:"weight" yaml:"weight"`
	Notes      string  `json:"notes,omitempty" yaml:"notes,omitempty"`
}

// LocationFeature is an interactive element at a location.
type LocationFeature struct {
	Name        string `json:"name" yaml:"name"`
	Description string `json:"description,omitempty" yaml:"description,omitempty"`
	Skill       string `json:"skill,omitempty" yaml:"skill,omitempty"`
	DC          int    `json:"dc,omitempty" yaml:"dc,omitempty"`
	Success     string `json:"success,omitempty" yaml:"success,omitempty"`
	Failure     string `json:"failure,omitempty" yaml:"failure,omitempty"`
}

// Indexes are auto-built lookup tables for fast queries.
type Indexes struct {
	ByCity              map[string][]string `json:"by_city,omitempty" yaml:"by_city,omitempty"`
	ByRegion            map[string][]string `json:"by_region,omitempty" yaml:"by_region,omitempty"`
	ByTag               map[string][]string `json:"by_tag,omitempty" yaml:"by_tag,omitempty"`
	ByCreatureHabitat   map[string][]string `json:"by_creature_habitat,omitempty" yaml:"by_creature_habitat,omitempty"`
	ByDistrict          map[string][]string `json:"by_district,omitempty" yaml:"by_district,omitempty"`
	ByFaction           map[string][]string `json:"by_faction,omitempty" yaml:"by_faction,omitempty"`
	NPCLocationIndex    map[string]string   `json:"npc_location_index,omitempty" yaml:"npc_location_index,omitempty"`
	LocationContentIndex map[string]string  `json:"location_content_index,omitempty" yaml:"location_content_index,omitempty"`
}

// ToolBinding binds a canonical world query tool for oracle use.
type ToolBinding struct {
	CanonicalID   string          `json:"canonical_id" yaml:"canonical_id"`
	Enabled       bool            `json:"enabled" yaml:"enabled"`
	Name          string          `json:"name" yaml:"name"`
	Description   string          `json:"description" yaml:"description"`
	Parameters    json.RawMessage `json:"parameters" yaml:"parameters"`
	Category      string          `json:"category,omitempty" yaml:"category,omitempty"`
	Preconditions []string        `json:"preconditions,omitempty" yaml:"preconditions,omitempty"`
	Examples      []ToolExample   `json:"examples,omitempty" yaml:"examples,omitempty"`
	Notes         string          `json:"notes,omitempty" yaml:"notes,omitempty"`
}

// ToolExample demonstrates a tool invocation.
type ToolExample struct {
	Title  string         `json:"title" yaml:"title"`
	Input  map[string]any `json:"input,omitempty" yaml:"input,omitempty"`
	Output string         `json:"output" yaml:"output"`
}

// OracleGuide advises when to query the world pack vs invent content.
type OracleGuide struct {
	Principles   []string        `json:"principles" yaml:"principles"`
	ToolPriority []string        `json:"tool_priority,omitempty" yaml:"tool_priority,omitempty"`
	AntiPatterns []string        `json:"anti_patterns,omitempty" yaml:"anti_patterns,omitempty"`
	Scenarios    []GuideScenario `json:"scenarios,omitempty" yaml:"scenarios,omitempty"`
}

// GuideScenario maps a situation to recommended tools.
type GuideScenario struct {
	Situation string   `json:"situation" yaml:"situation"`
	UseTools  []string `json:"use_tools" yaml:"use_tools"`
	Avoid     []string `json:"avoid,omitempty" yaml:"avoid,omitempty"`
	InventWhen string  `json:"invent_when,omitempty" yaml:"invent_when,omitempty"`
}

// EngineCompat maps worldpack tools to future engine hooks.
type EngineCompat struct {
	RoomType      string            `json:"room_type,omitempty" yaml:"room_type,omitempty"`
	NPCType       string            `json:"npc_type,omitempty" yaml:"npc_type,omitempty"`
	CreatureType  string            `json:"creature_type,omitempty" yaml:"creature_type,omitempty"`
	ToolMap       map[string]string `json:"tool_map,omitempty" yaml:"tool_map,omitempty"`
	Notes         string            `json:"notes,omitempty" yaml:"notes,omitempty"`
}
