package mistfall_coast

import (
	"github.com/theburrowhub/thaimaturgy/internal/worldpack"
)

func buildToolExamples(p *worldpack.Pack) {
	worldpack.BindToolFromCanonical(p, "get_location", "get_location", "Fetch a place by ID.", "geography", nil, nil)
	worldpack.BindToolFromCanonical(p, "get_npc", "get_npc", "Fetch an NPC.", "population", nil, nil)
	worldpack.BindToolFromCanonical(p, "roll_encounter_table", "roll_encounter_table", "Roll encounter table.", "encounters", nil, nil)
	worldpack.BindToolFromCanonical(p, "search_world", "search_world", "Search catalog.", "reference", nil, nil)
}

func buildOracleScenarios(p *worldpack.Pack) {
	p.OracleGuide.Scenarios = []worldpack.GuideScenario{
		{Situation: "Investigators arrive in Harrowport during a week-long fog", UseTools: []string{"get_city", "list_city_locations", "get_lore"}},
		{Situation: "Salt Ledger publishes a Harbor Board name", UseTools: []string{"get_npc", "get_faction", "search_world"}, Avoid: []string{"Inventing a new newspaper"}},
		{Situation: "Keeper Holt invites the party to the lighthouse", UseTools: []string{"get_location", "get_npc", "list_location_creatures"}},
		{Situation: "Low tide exposes the Black Lantern wreck", UseTools: []string{"roll_encounter_table", "get_creature", "get_lore"}},
	}
}
