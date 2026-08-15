package caribdus

import (
	"github.com/theburrowhub/thaimaturgy/internal/worldpack"
)

func buildToolExamples(p *worldpack.Pack) {
	worldpack.BindToolFromCanonical(p, "get_city", "Puerto Sombrío Overview", "Fetch Puerto Sombrío city record.", "geography", nil,
		[]worldpack.ToolExample{{Title: "Party docks", Input: map[string]any{"city_id": "puerto_sombrio"}, Output: "Districts, population, harbor locations."}})
	worldpack.BindToolFromCanonical(p, "list_city_locations", "List Sombrío Docks", "Locations in Muelle Viejo.", "geography", nil,
		[]worldpack.ToolExample{{Title: "Find tavern", Input: map[string]any{"city_id": "puerto_sombrio", "district_id": "muelle_viejo"}, Output: "taberna_ancla_podrida, astillero_negro"}})
	worldpack.BindToolFromCanonical(p, "get_npc", "Meet Capitán Storm", "Load privateer captain NPC.", "population", nil,
		[]worldpack.ToolExample{{Title: "Tavern contact", Input: map[string]any{"npc_id": "npc_valeria_storm"}, Output: "Personality, faction, stat block."}})
	worldpack.BindToolFromCanonical(p, "roll_encounter_table", "Reef crossing roll", "Roll coral reef table.", "encounters", nil,
		[]worldpack.ToolExample{{Title: "d10 = 7", Input: map[string]any{"table_id": "encounter_reef", "roll": 7}, Output: "1d4+1 reef sharks."}})
	worldpack.BindToolFromCanonical(p, "search_world", "Search smugglers", "Find smuggler references.", "reference", nil,
		[]worldpack.ToolExample{{Title: "Query smuggler", Input: map[string]any{"query": "contrabandista", "limit": 5}, Output: "npc_garfio_reyes, cala_contrabandistas, item_chart_shoals..."}})
}

func buildOracleScenarios(p *worldpack.Pack) {
	p.OracleGuide.Scenarios = append(p.OracleGuide.Scenarios,
		worldpack.GuideScenario{
			Situation:  "Party docks at Puerto Sombrío",
			UseTools:   []string{"get_city", "list_city_locations", "get_location", "list_location_npcs", "get_lore"},
			Avoid:      []string{"Inventing a new harbor district", "Replacing Taberna del Ancla Podrida with a generic tavern"},
			InventWhen: "Players seek a shop type not listed — invent name but reuse market or dock tags.",
		},
		worldpack.GuideScenario{
			Situation: "Random open-sea voyage day",
			UseTools:  []string{"roll_encounter_table", "get_creature", "describe_travel"},
			Avoid:     []string{"Custom naval encounters when encounter_open_sea exists"},
		},
		worldpack.GuideScenario{
			Situation: "Party enters Ghost Shoals mist",
			UseTools:  []string{"get_region", "roll_encounter_table", "get_lore", "get_creature"},
			Avoid:     []string{"Ignoring curse lore from lore_shoal_bells"},
		},
	)
}
