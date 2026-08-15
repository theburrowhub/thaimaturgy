package mistfall_coast

import (
	"github.com/theburrowhub/thaimaturgy/internal/worldpack"
)

func Build() *worldpack.Pack {
	p := worldpack.NewBaseWorld("mistfall_coast", "Mistfall Coast", worldpack.WorldMeta{
		SettingName:         "Mistfall Coast",
		Summary:             "Fog-wrapped coastal province: smugglers, reformers, and a drowned cult beneath the lighthouses.",
		SuggestedRulesystem: "d100",
		PlayableWith:        []string{"d100", "dnd5e"},
	})
	worldpack.SetSettingTone(p,
		"Late 19th century — gas lamps, telegrams, bolt-actions; superstition persists",
		"Investigation and dread; polite society over rotten pilings",
		"Mist clings to Harrowport year-round. Every storm washes up something that should have stayed down.",
		"investigation", "horror", "coastal", "d100",
	)
	worldpack.SetWorldRulesFull(p, worldpack.WorldRules{
		Magic:             "Rare — folk charms, drowned-cult rites, relics that cost Sanity. No academies.",
		Technology:        "Late Victorian — rail to Brackenford; electric lights in Harrowport's promenade.",
		DeathAndAfterlife: "Church burial; cultists speak of minds adrift in the fog bank.",
		Travel:            "Steam packet between towns; moor roads unsafe at night.",
	})
	worldpack.SetPoliticsFull(p, worldpack.Politics{
		Summary:     "Harbor Board tariffs, inland landowners blocking reform, smugglers arming both sides, Drowned Lantern cult in the lighthouse service.",
		MajorPowers: []string{"Harrowport Harbor Board", "Brackenford Council", "Mist Runners smugglers", "Drowned Lantern cult"},
		Conflicts:   []string{"Tariffs vs smuggling", "Missing fishers vs cover-ups", "Reform press vs corrupt officials"},
		LawAndOrder: "Constabulary in towns; fog moor is extralegal.",
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
