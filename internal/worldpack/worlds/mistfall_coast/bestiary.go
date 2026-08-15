package mistfall_coast

import (
	"github.com/theburrowhub/thaimaturgy/internal/domain"
	"github.com/theburrowhub/thaimaturgy/internal/worldpack"
)

func d100Profile(str, con, siz, dex, hp, skill, dmg, notes string) domain.StatBlock {
	return domain.StatBlock{
		Type:    "d100 profile",
		Traits:  []string{"STR " + str, "CON " + con, "SIZ " + siz, "DEX " + dex, "HP " + hp, skill, notes},
		Actions: []domain.Action{{Name: "Attack", Damage: dmg}},
	}
}

func dndFallback(name, cr string, hp, ac int, toHit, dmg string) domain.StatBlock {
	return domain.StatBlock{Type: "Humanoid", CR: cr, MaxHP: hp, AC: ac,
		Actions: []domain.Action{{Name: "Attack", ToHit: toHit, Damage: dmg}}}
}

func addMistCreature(p *worldpack.Pack, id, name string, d100 domain.StatBlock, habitats []string, notes, lore string, tags ...string) {
	worldpack.AddCreatureMulti(p, id, name, map[string]domain.StatBlock{
		"d100":  d100,
		"dnd5e": dndFallback(name, "1/2", 22, 12, "+3", "1d6+1"),
	}, habitats, notes, lore, tags...)
}

func buildBestiary(p *worldpack.Pack) {
	addMistCreature(p, "creature_constable", "Constable",
		d100Profile("55", "60", "65", "50", "12", "Firearms (Handgun) 50", "1d10", "Badge authority in city"),
		[]string{"urban"}, "Pairs patrol promenade.", "Standard Harrowport beat cop.", "humanoid")
	addMistCreature(p, "creature_smuggler", "Smuggler",
		d100Profile("50", "55", "60", "65", "11", "Stealth 60", "1d8+db knife", "Knows fog channels"),
		[]string{"urban", "coast"}, "2d4 near docks at night.", "Mist Runners rank-and-file.", "humanoid", "criminal")
	addMistCreature(p, "creature_cultist", "Drowned Lantern Cultist",
		d100Profile("45", "50", "55", "55", "10", "Occult 45", "1d6 ritual knife", "Sanity test on reveal"),
		[]string{"coast", "temple"}, "1d6 at tidal shrine.", "Hooded wax-coated robes.", "humanoid", "cult")
	addMistCreature(p, "creature_drowned_one", "Drowned One",
		d100Profile("60", "—", "65", "40", "14", "Fighting (Brawl) 55", "1d8+db claw", "Undead; fear first sight"),
		[]string{"coast", "wreck"}, "Rises from Black Lantern wreck.", "Barnacled former fishers.", "undead")
	addMistCreature(p, "creature_fog_lurker", "Fog Lurker",
		d100Profile("70", "50", "80", "35", "16", "Stealth 70", "1d10 tentacle", "Invisible in fog"),
		[]string{"fog", "coast"}, "One per failed Navigate roll.", "Not natural taxonomy.", "monstrosity")
	addMistCreature(p, "creature_mine_thing", "Mine Tunnel Thing",
		d100Profile("80", "60", "70", "30", "18", "Fighting (Brawl) 60", "1d10+db", "Darkness advantage"),
		[]string{"underground"}, "Shaft Seven if opened.", "Sings with miners' voices.", "aberration")
	addMistCreature(p, "creature_feral_dog", "Fogward Feral Pack",
		d100Profile("35", "45", "40", "55", "8", "Fighting (Brawl) 40", "1d6 bite", "Pack of 4–8"),
		[]string{"urban"}, "Slums after midnight.", "Rabies fear in Fogward.", "beast")
	addMistCreature(p, "creature_lighthouse_keeper", "Corrupted Keeper",
		d100Profile("50", "55", "60", "50", "12", "Occult 60", "1d8 lantern smash", "Knows service tunnels"),
		[]string{"lighthouse"}, "Solitary at Harrow Lighthouse.", "May still wear uniform.", "humanoid", "cult")
	addMistCreature(p, "creature_investigator", "Private Inquiry Agent",
		d100Profile("45", "50", "55", "60", "10", "Psychology 55", "1d6 cane", "Credit Rating 30"),
		[]string{"urban"}, "Social encounter.", "Template for allied NPCs.", "humanoid")
	addMistCreature(p, "creature_deep_fish", "Deep Mist Fish",
		d100Profile("65", "40", "50", "60", "13", "Swim 70", "1d8 bite", "Amphibious"),
		[]string{"coast", "tidal"}, "Washed up on flats.", "Bioluminescent eyes.", "beast")
}
