package mistfall_coast

import (
	"github.com/theburrowhub/thaimaturgy/internal/worldpack"
)

func buildLocationContents(p *worldpack.Pack) {
	worldpack.AddLocationContents(p, worldpack.LocationContents{
		LocationID: "anchor_and_needle", NPCIDs: []string{"npc_jonah_mist"}, EncounterTableID: "encounter_docks_night",
	})
	worldpack.AddLocationContents(p, worldpack.LocationContents{
		LocationID: "harrow_lighthouse", NPCIDs: []string{"npc_keeper_holt"},
		CreatureWeights: []worldpack.WeightedCreature{{CreatureID: "creature_lighthouse_keeper", Weight: 1}},
	})
	worldpack.AddLocationContents(p, worldpack.LocationContents{
		LocationID: "wreck_black_lantern", EncounterTableID: "encounter_salt_flats",
		CreatureWeights: []worldpack.WeightedCreature{{CreatureID: "creature_drowned_one", Weight: 3}},
	})
	worldpack.AddLocationContents(p, worldpack.LocationContents{
		LocationID: "the_salt_ledger", NPCIDs: []string{"npc_nadia_croft"}, ItemIDs: []string{"item_salt_ledger_press"},
	})
	worldpack.AddLocationContents(p, worldpack.LocationContents{
		LocationID: "sealed_mine_shaft", EncounterTableID: "encounter_mine",
	})
}

func buildEncounterTables(p *worldpack.Pack) {
	tables := []worldpack.EncounterTable{
		{ID: "encounter_docks_night", Name: "Harrowport Docks at Night", Context: "urban_night", Biome: "urban", Dice: "d100",
			Rows: []worldpack.EncounterTableRow{
				{Roll: "1-40", Result: "Smugglers", CreatureIDs: []string{"creature_smuggler"}, Quantity: "2d4"},
				{Roll: "41-65", Result: "Constables", CreatureIDs: []string{"creature_constable"}, Quantity: "1d4"},
				{Roll: "66-85", Result: "Feral dogs", CreatureIDs: []string{"creature_feral_dog"}, Quantity: "1d6"},
				{Roll: "86-100", Result: "Fog lurker", CreatureIDs: []string{"creature_fog_lurker"}, Quantity: "1"},
			}},
		{ID: "encounter_salt_flats", Name: "Salt Flats at Low Tide", Context: "coast", Biome: "coastal", Dice: "d100",
			Rows: []worldpack.EncounterTableRow{
				{Roll: "1-50", Result: "Drowned ones", CreatureIDs: []string{"creature_drowned_one"}, Quantity: "2d4"},
				{Roll: "51-75", Result: "Cultists", CreatureIDs: []string{"creature_cultist"}, Quantity: "1d6"},
				{Roll: "76-100", Result: "Deep mist fish", CreatureIDs: []string{"creature_deep_fish"}, Quantity: "1d4"},
			}},
		{ID: "encounter_mine", Name: "Shaft Seven Breach", Context: "underground", Biome: "underground", Dice: "d100",
			Rows: []worldpack.EncounterTableRow{
				{Roll: "1-70", Result: "Tunnel thing", CreatureIDs: []string{"creature_mine_thing"}, Quantity: "1"},
				{Roll: "71-100", Result: "Cult remnants", CreatureIDs: []string{"creature_cultist"}, Quantity: "2d4"},
			}},
		{ID: "encounter_fog_moor", Name: "Blackwood Fog Moor", Context: "wilderness", Biome: "highland", Dice: "d100",
			Rows: []worldpack.EncounterTableRow{
				{Roll: "1-60", Result: "Nothing but mist"},
				{Roll: "61-85", Result: "Fog lurker", CreatureIDs: []string{"creature_fog_lurker"}, Quantity: "1"},
				{Roll: "86-100", Result: "Cult patrol", CreatureIDs: []string{"creature_cultist"}, Quantity: "1d4"},
			}},
	}
	for _, t := range tables {
		worldpack.AddEncounterTable(p, t)
	}
}
