package shattered_vale

import (
	"github.com/theburrowhub/thaimaturgy/internal/worldpack"
)

// Build returns The Shattered Vale world pack.
func Build() *worldpack.Pack {
	p := worldpack.NewBaseWorld("shattered_vale", "The Shattered Vale", worldpack.WorldMeta{
		SettingName:         "The Shattered Vale",
		SuggestedRulesystem: "dnd5e",
		PlayableWith:        []string{"dnd5e", "savage_worlds", "d100"},
	})
	worldpack.SetSettingTone(p,
		"Late medieval fantasy, five years after the Shattering",
		"Heroic with creeping dread; trade hubs bustle while wilderness grows feral",
		"A river-valley region fractured by a magical cataclysm. City-states cling to roads and rivers while monsters reclaim the wilds.",
		"fantasy", "riverlands", "sandbox",
	)

	worldpack.SetWorldRulesFull(p, worldpack.WorldRules{
		Magic:             "Common — divine clerics, arcane wizards, and druidic rites; ley-line scars from the Shattering cause wild surges.",
		Technology:        "Late medieval — castles, mills, crossbows; no gunpowder.",
		DeathAndAfterlife: "Souls reach the Outer Planes unless trapped by necromancy.",
		Travel:            "Roads between city-states; river barges; wilderness is dangerous.",
	})
	worldpack.SetPoliticsFull(p, worldpack.Politics{
		Summary:     "Fragmented city-states, guild leagues, and feudal holds compete after the Shattering.",
		MajorPowers: []string{"Merchants' League (Millhaven)", "Order of the Dawn", "Ironhold Wardenate", "Red Hand Bandits"},
		Conflicts:   []string{"League tariffs vs Ironhold tolls", "Dawn vs Undercrypt necromancy", "Red Hand vs town guards"},
		LawAndOrder: "Town charters and temple courts in cities; wilderness is self-help.",
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
