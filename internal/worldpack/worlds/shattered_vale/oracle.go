package shattered_vale

import (
	"github.com/theburrowhub/thaimaturgy/internal/worldpack"
)

func buildToolExamples(p *worldpack.Pack) {
	worldpack.BindToolFromCanonical(p, "get_city", "Millhaven Overview", "Fetch Millhaven city record.", "geography", nil,
		[]worldpack.ToolExample{{Title: "Party arrives", Input: map[string]any{"city_id": "millhaven"}, Output: "Returns districts, population, and location IDs."}})
	worldpack.BindToolFromCanonical(p, "list_city_locations", "List Millhaven Harbor", "Locations in harbor district.", "geography", nil,
		[]worldpack.ToolExample{{Title: "Find the docks", Input: map[string]any{"city_id": "millhaven", "district_id": "harbor"}, Output: "river_docks, sunlit_lighthouse"}})
	worldpack.BindToolFromCanonical(p, "get_npc", "Meet Captain Thorne", "Load guard captain NPC.", "population", nil,
		[]worldpack.ToolExample{{Title: "Report crime", Input: map[string]any{"npc_id": "npc_mira_thorne"}, Output: "Personality, stat block, barracks location."}})
	worldpack.BindToolFromCanonical(p, "roll_encounter_table", "Forest travel roll", "Roll Whisperwood table.", "encounters", nil,
		[]worldpack.ToolExample{{Title: "d12 = 5", Input: map[string]any{"table_id": "encounter_whisperwood", "roll": 5}, Output: "2d4 goblins ambush."}})
	worldpack.BindToolFromCanonical(p, "search_world", "Search for bandits", "Find Red Hand references.", "reference", nil,
		[]worldpack.ToolExample{{Title: "Query red hand", Input: map[string]any{"query": "Red Hand", "limit": 5}, Output: "npc_cassian_red, lore_red_hand, cutpurse_alley..."}})
}

func buildOracleScenarios(p *worldpack.Pack) {
	p.OracleGuide.Scenarios = []worldpack.GuideScenario{
		{
			Situation:  "Party enters Millhaven for the first time",
			UseTools:   []string{"get_city", "list_city_locations", "get_location", "get_lore"},
			Avoid:      []string{"Inventing a new city quarter"},
			InventWhen: "Players ask for a shop type not listed — then invent name but reuse market tags.",
		},
		{
			Situation: "Random forest encounter on the King's Road",
			UseTools:  []string{"roll_encounter_table", "get_creature", "filter_creatures_by_habitat"},
			Avoid:     []string{"Making up new monster stats"},
		},
		{
			Situation: "Player asks who is in the tavern",
			UseTools:  []string{"get_location", "list_location_npcs", "get_npc"},
			Avoid:     []string{"Generating a random tavernkeeper when Gilded Anchor is established"},
		},
		{
			Situation: "Party explores Undercrypt level 1",
			UseTools:  []string{"get_location", "list_location_creatures", "roll_encounter_table", "get_creature"},
			Avoid:     []string{"Skipping undead stat blocks"},
		},
		{
			Situation:  "Player wants to buy healing potions",
			UseTools:   []string{"list_location_items", "get_item", "get_location"},
			InventWhen: "Only if party is far from authored shops.",
		},
		{
			Situation: "Travel from Millhaven to Thornwall",
			UseTools:  []string{"describe_travel", "get_region", "roll_encounter_table"},
			Avoid:     []string{"Teleporting encounters without road table"},
		},
		{
			Situation: "Party searches for Red Hand hideout",
			UseTools:  []string{"search_world", "get_faction", "get_lore", "find_nearby_locations"},
			Avoid:     []string{"Relocating Cassian without story reason"},
		},
	}
}
