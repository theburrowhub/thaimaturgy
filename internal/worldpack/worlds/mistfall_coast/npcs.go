package mistfall_coast

import (
	"github.com/theburrowhub/thaimaturgy/internal/worldpack"
)

func buildNPCs(p *worldpack.Pack) {
	npcs := []worldpack.WorldNPC{
		{ID: "npc_inspector_vale", Name: "Inspector Cordelia Vale", Role: "constabulary",
			Appearance: "Sharp grey uniform, scarred knuckles, never unarmed.", Personality: "Dry, relentless, hates the Board.",
			Motivations: "Close the missing fishers case before the Mayor buries it.", Secrets: "Took Mist Runner bribes once; repaid in double.",
			DefaultLocation: "constabulary_hq", FactionID: "harbor_board", Tags: []string{"law", "ally"}},
		{ID: "npc_elias_morse", Name: "Father Elias Morse", Role: "priest",
			Appearance: "Tired eyes, soup-stained cassock.", Personality: "Gentle public face, steel in private.",
			Motivations: "Protect Fogward parishioners.", Secrets: "Hides cult victims in crypt.",
			DefaultLocation: "fogward_chapel", Tags: []string{"faith"}},
		{ID: "npc_nadia_croft", Name: "Nadia Croft", Role: "journalist",
			Appearance: "Ink-stained fingers, practical boots.", Personality: "Bold questions, faster feet.",
			Motivations: "Publish proof of Board corruption.", Secrets: "Source inside lighthouse service.",
			DefaultLocation: "the_salt_ledger", FactionID: "salt_ledger", Tags: []string{"investigation"}},
		{ID: "npc_jonah_mist", Name: "Jonah 'Two-Tide' Marsh", Role: "smuggler",
			Appearance: "Tarred coat, gold earring, fog-grey beard.", Personality: "Jokes through threats.",
			Motivations: "Keep runners free of Crown hangmen.", Secrets: "Brother is a lighthouse keeper.",
			DefaultLocation: "anchor_and_needle", FactionID: "mist_runners", Tags: []string{"criminal"}},
		{ID: "npc_keeper_holt", Name: "Keeper Silas Holt", Role: "lighthouse keeper",
			Appearance: "Burned left hand, uniform always pressed.", Personality: "Courteous, evasive.",
			Motivations: "Maintain the beam.", Secrets: "Drowned Lantern initiate.",
			DefaultLocation: "harrow_lighthouse", FactionID: "drowned_lantern", Tags: []string{"cult"}},
		{ID: "npc_mayor_penn", Name: "Mayor Aldous Penn", Role: "politician",
			Appearance: "Silk waistcoat, constant smile.", Personality: "Charming, calculating.",
			Motivations: "Re-election and Harbor Board donations.", Secrets: "Signed disappearances as 'accidents'.",
			DefaultLocation: "customs_house", FactionID: "harbor_board", Tags: []string{"noble"}},
		{ID: "npc_dr_sallow", Name: "Dr. Irene Sallow", Role: "physician",
			Appearance: "Clinical apron, steady hands.", Personality: "Clinical empathy.",
			Motivations: "Document fog-related madness.", Secrets: "Experimenting with nerve tonic doses.",
			DefaultLocation: "constabulary_hq", Tags: []string{"medicine"}},
		{ID: "npc_lila_bracken", Name: "Lila Bracken", Role: "innkeeper",
			Appearance: "Broad-shouldered, keys always jingling.", Personality: "Gossip hub, loyal to paying guests.",
			Motivations: "Keep Moor Cock profitable.", Secrets: "Passes council minutes to Nadia.",
			DefaultLocation: "bracken_inn", Tags: []string{"social"}},
	}
	for _, n := range npcs {
		worldpack.AddNPC(p, n)
	}
}
