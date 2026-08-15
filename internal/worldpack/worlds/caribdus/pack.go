package caribdus

import (
	"github.com/theburrowhub/thaimaturgy/internal/worldpack"
)

// Build returns the Caribdus flooded-archipelago world pack.
func Build() *worldpack.Pack {
	p := worldpack.NewBaseWorld("caribdus", "Caribdus", worldpack.WorldMeta{
		SettingName:         "Caribdus",
		Summary:             "A drowned archipelago where colonial fleets, pirate republics, and sea-witch covens fight over reef-choked waters and cursed treasure.",
		SuggestedRulesystem: "savage_worlds",
		PlayableWith:        []string{"savage_worlds", "dnd5e", "d100"},
	})

	worldpack.SetSettingTone(p,
		"Sailing age — flintlocks, powder, and rope; no heavy industry",
		"Swashbuckling dread; salt spray, superstition, and sudden violence",
		"Caribdus is a chain of storm-battered isles half-swallowed by the Deluge. Every cove hides a flag, every reef hides teeth.",
		"nautical", "pirates", "sea-witch", "colonial",
	)

	worldpack.SetWorldRulesFull(p, worldpack.WorldRules{
		Magic:      "Common — sea-witch hedge magic, cursed relics from the Deluge, and tide-bound rituals. Arcane academies are absent; power smells of brine and blood.",
		Technology: "Sailing age — flintlock pistols, cutlasses, schooners, and diving bells. No steam, no mass production; shipyards hand-fit every hull.",
		Travel:     "Open sea between isles; reef pilots required in the archipelago; Ghost Shoals warp compasses.",
	})

	worldpack.SetPoliticsFull(p, worldpack.Politics{
		Summary: "Colonial armadas claim harbors with cannon and charter; pirate republics vote captains by steel and spoils; sea-witch covens trade storms for souls along the Ghost Shoals.",
		MajorPowers: []string{
			"Armada del Rey (Fuerte Almirez)",
			"República de Perla Azul",
			"Capitanes Libres (Puerto Sombrío)",
			"Coven del Mar Espectral",
		},
		Conflicts: []string{
			"Tariffs vs smuggling routes",
			"Pirate raids vs colonial patrols",
			"Sea-witch bargains vs temple inquisitors",
		},
	})

	buildRegions(p)
	buildMaps(p)
	buildCitiesAndLocations(p)
	buildFactions(p)
	buildLore(p)
	buildBestiary(p)
	buildItems(p)
	buildNPCs(p)
	buildLocationContents(p)
	buildEncounterTables(p)
	buildToolExamples(p)
	buildOracleScenarios(p)

	worldpack.BuildIndexes(p)
	return p
}
