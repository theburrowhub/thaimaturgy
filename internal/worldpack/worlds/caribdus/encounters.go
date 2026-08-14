package caribdus

import (
	"github.com/theburrowhub/thaimaturgy/internal/worldpack"
)

func buildLocationContents(p *worldpack.Pack) {
	contents := []worldpack.LocationContents{
		{
			LocationID:       "taberna_ancla_podrida",
			NPCIDs:           []string{"npc_paco_tabernero", "npc_valeria_storm", "npc_cuervo_salazar", "npc_isla_marinera"},
			ItemIDs:          []string{"item_grog", "item_rum_fine"},
			EncounterTableID: "encounter_port_night",
		},
		{
			LocationID: "astillero_negro",
			NPCIDs:     []string{"npc_toro_contramaestre"},
			ItemIDs:    []string{"item_sailor_rope", "item_grappling_hook"},
		},
		{
			LocationID: "aduana_sombrio",
			NPCIDs:     []string{"npc_envoy_sombrio"},
			ItemIDs:    []string{"item_compass"},
		},
		{
			LocationID: "fortin_colonial",
			CreatureWeights: []worldpack.WeightedCreature{
				{CreatureID: "creature_crown_marine", Weight: 1.0, Notes: "Day patrol"},
			},
		},
		{
			LocationID: "templo_mareas",
			NPCIDs:     []string{"npc_coral_priestess"},
			ItemIDs:    []string{"item_witch_charm"},
		},
		{
			LocationID: "palacio_gobernador",
			NPCIDs:     []string{"npc_mateo_almirez"},
		},
		{
			LocationID: "cuartel_almirez",
			NPCIDs:     []string{"npc_almirante_ribera"},
			CreatureWeights: []worldpack.WeightedCreature{
				{CreatureID: "creature_crown_marine", Weight: 1.0},
			},
		},
		{
			LocationID: "cala_contrabandistas",
			NPCIDs:     []string{"npc_garfio_reyes"},
			ItemIDs:    []string{"item_chart_shoals", "item_flintlock"},
			CreatureWeights: []worldpack.WeightedCreature{
				{CreatureID: "creature_pirate_crew", Weight: 0.6},
			},
		},
		{
			LocationID: "arrecife_coral",
			CreatureWeights: []worldpack.WeightedCreature{
				{CreatureID: "creature_reef_shark", Weight: 0.5},
				{CreatureID: "creature_giant_crab", Weight: 0.3},
				{CreatureID: "creature_jellyfish_swarm", Weight: 0.2},
			},
			EncounterTableID: "encounter_reef",
		},
		{
			LocationID: "choza_bruja_mar",
			NPCIDs:     []string{"npc_marisela_bruma"},
			ItemIDs:    []string{"item_witch_charm"},
			CreatureWeights: []worldpack.WeightedCreature{
				{CreatureID: "creature_sea_witch_familiar", Weight: 1.0},
			},
		},
		{
			LocationID: "naufragio_espectral",
			CreatureWeights: []worldpack.WeightedCreature{
				{CreatureID: "creature_ghost_pirate", Weight: 0.4},
				{CreatureID: "creature_drowned_sailor", Weight: 0.6},
			},
			EncounterTableID: "encounter_ghost_shoals",
		},
		{
			LocationID: "fosa_abisal",
			CreatureWeights: []worldpack.WeightedCreature{
				{CreatureID: "creature_leviathan_spawn", Weight: 0.3},
				{CreatureID: "creature_giant_octopus", Weight: 0.4},
			},
			EncounterTableID: "encounter_deep_water",
		},
		{
			LocationID: "templo_sumergido",
			ItemIDs:    []string{"item_cursed_pearl"},
			CreatureWeights: []worldpack.WeightedCreature{
				{CreatureID: "creature_barnacle_zombie", Weight: 0.7},
				{CreatureID: "creature_drowned_sailor", Weight: 0.3},
			},
		},
		{
			LocationID:       "faro_tormenta",
			ItemIDs:          []string{"item_spyglass", "item_compass"},
			EncounterTableID: "encounter_open_sea",
		},
	}
	for _, lc := range contents {
		worldpack.AddLocationContents(p, lc)
	}
}

func buildEncounterTables(p *worldpack.Pack) {
	tables := []worldpack.EncounterTable{
		{
			ID: "encounter_open_sea", Name: "Open Sea Voyage", Context: "ocean", Biome: "open_sea", Dice: "d12",
			Description: "Days under sail between islands.",
			Rows: []worldpack.EncounterTableRow{
				{Roll: "1-3", Result: "Fair winds; no encounter."},
				{Roll: "4-5", Result: "Merchant sloop (trade rumors or piracy?)."},
				{Roll: "6-7", Result: "1d4 reef sharks circle the hull.", CreatureIDs: []string{"creature_reef_shark"}, Quantity: "1d4"},
				{Roll: "8", Result: "Jellyfish bloom — Navigation hazard.", CreatureIDs: []string{"creature_jellyfish_swarm"}, Quantity: "1"},
				{Roll: "9-10", Result: "Pirate schooner intercepts.", CreatureIDs: []string{"creature_pirate_crew"}, Quantity: "6"},
				{Roll: "11", Result: "Crown patrol demands papers.", CreatureIDs: []string{"creature_crown_marine"}, Quantity: "8"},
				{Roll: "12", Result: "Leviathan shadow beneath the keel.", CreatureIDs: []string{"creature_leviathan_spawn"}, Quantity: "1"},
			},
		},
		{
			ID: "encounter_port_night", Name: "Port Night (Puerto Sombrío)", Context: "urban_night", Biome: "urban", DistrictID: "muelle_viejo", Dice: "d8",
			Rows: []worldpack.EncounterTableRow{
				{Roll: "1-2", Result: "Drunk sailors; no threat."},
				{Roll: "3-4", Result: "Pickpocket pair (use pirate crew stats).", CreatureIDs: []string{"creature_pirate_crew"}, Quantity: "2"},
				{Roll: "5-6", Result: "Smuggler offload interrupted.", CreatureIDs: []string{"creature_pirate_crew"}, Quantity: "1d4+2"},
				{Roll: "7", Result: "Crown patrol sweep.", CreatureIDs: []string{"creature_crown_marine"}, Quantity: "1d4"},
				{Roll: "8", Result: "Valeria Storm recruiting in the tavern.", Notes: "Social; see npc_valeria_storm."},
			},
		},
		{
			ID: "encounter_reef", Name: "Coral Reef Passage", Context: "reef", Biome: "reef", Dice: "d10",
			Rows: []worldpack.EncounterTableRow{
				{Roll: "1-3", Result: "Clear channel; colorful fish only."},
				{Roll: "4-5", Result: "Giant crabs on the flats.", CreatureIDs: []string{"creature_giant_crab"}, Quantity: "1d2"},
				{Roll: "6-7", Result: "Reef sharks drawn by blood.", CreatureIDs: []string{"creature_reef_shark"}, Quantity: "1d4+1"},
				{Roll: "8", Result: "Giant octopus in a wreck.", CreatureIDs: []string{"creature_giant_octopus"}, Quantity: "1"},
				{Roll: "9", Result: "Smuggler cache and lookouts.", Notes: "item_chart_shoals possible."},
				{Roll: "10", Result: "Jellyfish swarm blocks the channel.", CreatureIDs: []string{"creature_jellyfish_swarm"}, Quantity: "1"},
			},
		},
		{
			ID: "encounter_deep_water", Name: "Deep Trench Dive", Context: "deep", Biome: "deep", Dice: "d8",
			Rows: []worldpack.EncounterTableRow{
				{Roll: "1", Result: "Bioluminescent calm."},
				{Roll: "2-3", Result: "Giant octopus on the temple wall.", CreatureIDs: []string{"creature_giant_octopus"}, Quantity: "1"},
				{Roll: "4-5", Result: "Barnacle zombies on the descent line.", CreatureIDs: []string{"creature_barnacle_zombie"}, Quantity: "2d4"},
				{Roll: "6", Result: "Drowned sailors rise from a crevice.", CreatureIDs: []string{"creature_drowned_sailor"}, Quantity: "2d6"},
				{Roll: "7", Result: "Black pearl lodged in coral — cursed.", Notes: "item_cursed_pearl"},
				{Roll: "8", Result: "Leviathan spawn passes below.", CreatureIDs: []string{"creature_leviathan_spawn"}, Quantity: "1"},
			},
		},
		{
			ID: "encounter_ghost_shoals", Name: "Ghost Shoals Mist", Context: "mist", Biome: "ghost_shoals", Dice: "d10",
			Rows: []worldpack.EncounterTableRow{
				{Roll: "1-2", Result: "Fog thickens; bells distant."},
				{Roll: "3-4", Result: "Drowned sailors wade from mist.", CreatureIDs: []string{"creature_drowned_sailor"}, Quantity: "2d4"},
				{Roll: "5-6", Result: "Bruma Marina offers a bargain.", CreatureIDs: []string{"creature_bruma_marina"}, Quantity: "1", Notes: "Social or combat."},
				{Roll: "7-8", Result: "Ghost pirate officers.", CreatureIDs: []string{"creature_ghost_pirate", "creature_drowned_sailor"}, Quantity: "1 + 2d4"},
				{Roll: "9", Result: "Sea witch familiar watches.", CreatureIDs: []string{"creature_sea_witch_familiar"}, Quantity: "1"},
				{Roll: "10", Result: "Naufragio Espectral materializes.", Notes: "Major set-piece."},
			},
		},
		{
			ID: "encounter_perla_docks", Name: "Perla Azul Docks (Day)", Context: "urban", Biome: "coast", Dice: "d6",
			Rows: []worldpack.EncounterTableRow{
				{Roll: "1-2", Result: "Pearl auction crowd; no threat."},
				{Roll: "3-4", Result: "Giant crabs in the shallows.", CreatureIDs: []string{"creature_giant_crab"}, Quantity: "1d2"},
				{Roll: "5", Result: "Corsair recruiters vs Crown press gang.", CreatureIDs: []string{"creature_pirate_crew", "creature_crown_marine"}, Quantity: "4 each"},
				{Roll: "6", Result: "Sacerdotisa Coral seeks divers for a ritual.", Notes: "Social; npc_coral_priestess."},
			},
		},
	}
	for _, t := range tables {
		worldpack.AddEncounterTable(p, t)
	}
}
