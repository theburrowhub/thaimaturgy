package caribdus

import (
	"github.com/theburrowhub/thaimaturgy/internal/domain"
	"github.com/theburrowhub/thaimaturgy/internal/worldpack"
)

func addCaribdusCreature(p *worldpack.Pack, id, name string, sw, dnd domain.StatBlock, habitats []string, encounterNotes, lore string, tags ...string) {
	worldpack.AddCreatureMulti(p, id, name, map[string]domain.StatBlock{
		"savage_worlds": sw,
		"dnd5e":         dnd,
	}, habitats, encounterNotes, lore, tags...)
}

func dndBeast(cr string, hp, ac int, toHit, dmg string) domain.StatBlock {
	return domain.StatBlock{Type: "Beast", CR: cr, MaxHP: hp, AC: ac,
		Actions: []domain.Action{{Name: "Attack", ToHit: toHit, Damage: dmg}}}
}

func dndHumanoid(cr string, hp, ac int, toHit, dmg string, traits ...string) domain.StatBlock {
	return domain.StatBlock{Type: "Humanoid", CR: cr, MaxHP: hp, AC: ac,
		Traits: traits, Actions: []domain.Action{{Name: "Attack", ToHit: toHit, Damage: dmg}}}
}

func dndUndead(cr string, hp, ac int, toHit, dmg string) domain.StatBlock {
	return domain.StatBlock{Type: "Undead", CR: cr, MaxHP: hp, AC: ac,
		Traits: []string{"Undead"}, Actions: []domain.Action{{Name: "Attack", ToHit: toHit, Damage: dmg}}}
}

func dndMonstrosity(cr string, hp, ac int, toHit, dmg string) domain.StatBlock {
	return domain.StatBlock{Type: "Monstrosity", CR: cr, MaxHP: hp, AC: ac,
		Actions: []domain.Action{{Name: "Attack", ToHit: toHit, Damage: dmg}}}
}

func buildBestiary(p *worldpack.Pack) {
	addCaribdusCreature(p, "creature_reef_shark", "Reef Shark",
		savageProfile("Normal", "A", "F", "6", "8", "Swim 12", "d6 bite", "Pack hunter"),
		dndBeast("1", 22, 12, "+3", "1d8+1"), []string{"reef", "coast", "open_sea"},
		"1d4+1 in bloodied water near reefs.", "Caribdus sailors call them red fins.", "beast", "aquatic")
	addCaribdusCreature(p, "creature_giant_octopus", "Giant Octopus",
		savageProfile("Large", "A", "G", "8", "12", "Swim 8", "2d6 tentacle", "Grapple on raise"),
		dndMonstrosity("1", 52, 11, "+5", "2d6+3"), []string{"reef", "deep", "ship"},
		"One per sunken wreck or night dive.", "Smugglers train juveniles poorly.", "beast", "aquatic")
	addCaribdusCreature(p, "creature_drowned_sailor", "Drowned Sailor",
		savageProfile("Normal", "A", "F", "6", "8", "Swim 6", "d6 rust-cutlass", "Undead"),
		dndUndead("1/2", 22, 11, "+3", "1d6+1"), []string{"ghost_shoals", "wreck", "open_sea"},
		"2d6 rising from spectral wreck at dusk.", "Crew of the Naufragio Espectral.", "undead", "aquatic")
	addCaribdusCreature(p, "creature_bruma_marina", "Bruma Marina",
		savageProfile("Normal", "A", "G", "8", "10", "Swim 8", "d6 claws", "Sea hag analog"),
		dndMonstrosity("2", 45, 13, "+4", "1d8+2"), []string{"ghost_shoals", "mist"},
		"Solitary in shoals; with 1d4 drowned sailors.", "Covens deny kinship.", "monstrosity", "sea-witch")
	addCaribdusCreature(p, "creature_pirate_crew", "Pirate Crew",
		savageProfile("Normal", "A", "F", "6", "6", "Pace 6", "d6 cutlass", "Wild Attack"),
		dndHumanoid("1/8", 11, 12, "+3", "1d6+1", "Sea Legs"), []string{"urban", "coast", "open_sea"},
		"Crew of 6-12 with one Wild Card captain.", "Mix of Corsair Council and freelancers.", "humanoid", "pirate")
	addCaribdusCreature(p, "creature_giant_crab", "Giant Crab",
		savageProfile("Large", "A", "G", "8", "14", "Pace 6", "2d6 pincer", "Hard shell"),
		dndBeast("1/2", 26, 15, "+3", "2d6+1"), []string{"reef", "coast"},
		"1-2 on tidal flats or pearl beds.", "Perla Azul divers carry hammers.", "beast", "aquatic")
	addCaribdusCreature(p, "creature_jellyfish_swarm", "Jellyfish Swarm",
		savageProfile("Large", "A", "G", "6", "8", "Swim 4", "d4 sting", "Poison fatigue"),
		dndBeast("1/2", 18, 11, "+2", "1d4"), []string{"reef", "open_sea", "deep"},
		"Swarm in warm currents.", "Called crown lace by Almirez marines.", "beast", "aquatic")
	addCaribdusCreature(p, "creature_ghost_pirate", "Ghost Pirate",
		savageProfile("Normal", "A", "G", "8", "10", "Pace 6", "d8 ethereal cutlass", "Undead"),
		dndUndead("3", 45, 13, "+5", "1d8+2"), []string{"ghost_shoals", "wreck"},
		"1 Wild Card with 1d6 crew shadows.", "Officers of the Naufragio Espectral.", "undead", "pirate")
	addCaribdusCreature(p, "creature_leviathan_spawn", "Leviathan Spawn",
		savageProfile("Huge", "A", "G", "12", "24", "Swim 10", "3d6 bite", "Swallow whole"),
		dndMonstrosity("5", 95, 14, "+7", "3d6+4"), []string{"deep", "open_sea"},
		"Solitary near trench; triggers Fear.", "Admirals deny reports.", "monstrosity", "horror")
	addCaribdusCreature(p, "creature_barnacle_zombie", "Barnacle Zombie",
		savageProfile("Normal", "A", "F", "6", "10", "Pace 4", "d6 slam", "Slow undead"),
		dndUndead("1/2", 22, 8, "+3", "1d6+1"), []string{"wreck", "ghost_shoals", "ship"},
		"2d4 aboard abandoned hulks.", "Failsafe guardians for smuggler scuttling.", "undead", "aquatic")
	addCaribdusCreature(p, "creature_sea_witch_familiar", "Sea Witch Familiar",
		savageProfile("Small", "A", "G", "6", "6", "Fly 8", "d4 talons", "Familiar"),
		dndBeast("0", 5, 12, "+2", "1d4"), []string{"ghost_shoals", "urban"},
		"With Marisela or temple envoys.", "Osprey, heron, or skeletal gull.", "beast", "familiar")
	addCaribdusCreature(p, "creature_crown_marine", "Crown Marine",
		savageProfile("Normal", "A", "F", "6", "8", "Pace 6", "d8 flintlock", "Volley fire"),
		dndHumanoid("1/2", 32, 14, "+4", "2d8", "Marine discipline"), []string{"urban", "fort", "naval"},
		"Squad of 4-8 with sergeant Wild Card.", "Fuerte Almirez issue.", "humanoid", "military")
}

func savageProfile(size, agility, spirit, fighting, strength, pace, damage, notes string) domain.StatBlock {
	return domain.StatBlock{
		Size:  size,
		Type:  "Savage Worlds profile",
		Speed: pace,
		Traits: []string{
			"Agility " + agility,
			"Spirit " + spirit,
			"Strength " + strength,
			"Fighting " + fighting,
			notes,
		},
		Actions: []domain.Action{{Name: "Primary", Damage: damage}},
	}
}

func captainStatBlock() domain.StatBlock {
	return domain.StatBlock{
		Type: "Humanoid", CR: "3",
		AC: 15, MaxHP: 45, Speed: "30 ft.",
		Abilities: domain.AbilityScores{STR: 14, DEX: 16, CON: 12, INT: 12, WIS: 11, CHA: 14},
		Traits:    []string{"Parry +1", "Sea Legs"},
		Actions:   []domain.Action{{Name: "Cutlass", ToHit: "+5", Damage: "1d8+2 slashing"}, {Name: "Flintlock", ToHit: "+5", Damage: "2d6+1 piercing"}},
	}
}

func seaWitchStatBlock() domain.StatBlock {
	return domain.StatBlock{
		Type: "Humanoid", CR: "5",
		AC: 13, MaxHP: 52, Speed: "30 ft., swim 30 ft.",
		Abilities: domain.AbilityScores{STR: 8, DEX: 14, CON: 12, INT: 14, WIS: 16, CHA: 16},
		Traits:    []string{"Sea witch magic", "Tide sense 60 ft."},
		Actions:   []domain.Action{{Name: "Claws", ToHit: "+4", Damage: "1d6+1 slashing"}, {Name: "Hex", Description: "Target −1 all rolls until end of next turn (1/turn)"}},
	}
}

func pirateStatBlock() domain.StatBlock {
	return domain.StatBlock{
		Type: "Humanoid", CR: "1",
		AC: 13, MaxHP: 22, Speed: "30 ft.",
		Abilities: domain.AbilityScores{STR: 12, DEX: 14, CON: 12, INT: 10, WIS: 10, CHA: 10},
		Actions:   []domain.Action{{Name: "Cutlass", ToHit: "+4", Damage: "1d6+1 slashing"}},
	}
}

func marineOfficerStatBlock() domain.StatBlock {
	return domain.StatBlock{
		Type: "Humanoid", CR: "4",
		AC: 16, MaxHP: 58, Speed: "30 ft.",
		Abilities: domain.AbilityScores{STR: 14, DEX: 12, CON: 14, INT: 12, WIS: 13, CHA: 12},
		Traits:    []string{"Tactical volley", "Marine discipline"},
		Actions:   []domain.Action{{Name: "Musket", ToHit: "+4", Damage: "2d8 piercing"}, {Name: "Sabre", ToHit: "+5", Damage: "1d8+2 slashing"}},
	}
}

func ptrStatBlock(sb domain.StatBlock) *domain.StatBlock { return &sb }
