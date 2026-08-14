package shattered_vale

import (
	"github.com/theburrowhub/thaimaturgy/internal/worldpack"
)

func buildLocationContents(p *worldpack.Pack) {
	contents := []worldpack.LocationContents{
		{
			LocationID: "millhaven_market",
			NPCIDs:     []string{"npc_alden_cross", "npc_durn_kettle"},
			ItemIDs:    []string{"item_rations", "item_torch", "item_rope"},
			Features: []worldpack.LocationFeature{
				{Name: "Pickpocket bustle", Description: "Perception DC 14 spots a thief.", Skill: "Perception", DC: 14, Success: "Spot the thief before losing 1d6 sp.", Failure: "Lose 1d6 sp or chase scene."},
			},
		},
		{
			LocationID: "the_gilded_anchor",
			NPCIDs:     []string{"npc_tomas_gull"},
			ItemIDs:    []string{"item_healing_potion"},
			Features: []worldpack.LocationFeature{
				{Name: "Hidden smuggler cellar", Skill: "Investigation", DC: 15, Success: "Find route to Cutpurse Alley.", Failure: "Tomas denies everything."},
			},
		},
		{
			LocationID: "temple_of_dawn",
			NPCIDs:     []string{"npc_lyra_dawn"},
			ItemIDs:    []string{"item_dawn_amulet", "item_healers_kit"},
		},
		{
			LocationID: "millhaven_barracks",
			NPCIDs:     []string{"npc_mira_thorne", "npc_brick_holt"},
			ItemIDs:    []string{"item_spear", "item_chain_shirt", "item_shield"},
		},
		{
			LocationID: "river_docks",
			NPCIDs:     []string{"npc_fenn_reed"},
			ItemIDs:    []string{"item_bag_of_trinkets"},
			CreatureWeights: []worldpack.WeightedCreature{
				{CreatureID: "creature_commoner", Weight: 0.7, Notes: "Dockworkers"},
				{CreatureID: "creature_bandit", Weight: 0.2, Notes: "Night smugglers"},
			},
		},
		{
			LocationID:       "cutpurse_alley",
			NPCIDs:           []string{"npc_sable_quinn"},
			ItemIDs:          []string{"item_thieves_tools", "item_red_hand_mask"},
			EncounterTableID: "encounter_urban_night",
		},
		{
			LocationID: "millhaven_town_hall",
			NPCIDs:     []string{"npc_eldric_vane"},
		},
		{
			LocationID: "ironhold_smithy",
			NPCIDs:     []string{"npc_helga_stone"},
			ItemIDs:    []string{"item_smith_hammer", "item_longsword", "item_scale_mail"},
		},
		{
			LocationID: "ironhold_keep",
			NPCIDs:     []string{"npc_gareth_ironhold"},
			CreatureWeights: []worldpack.WeightedCreature{
				{CreatureID: "creature_guard", Weight: 1.0},
			},
		},
		{
			LocationID: "thornwall_gatehouse",
			NPCIDs:     []string{"npc_jessa_marrow"},
		},
		{
			LocationID: "whisperwood_grove",
			NPCIDs:     []string{"npc_nim_willow"},
			ItemIDs:    []string{"item_moonpetal_herb"},
			CreatureWeights: []worldpack.WeightedCreature{
				{CreatureID: "creature_wolf", Weight: 0.4},
				{CreatureID: "creature_goblin", Weight: 0.3},
				{CreatureID: "creature_bugbear", Weight: 0.1},
			},
			EncounterTableID: "encounter_whisperwood",
		},
		{
			LocationID: "northern_marches_ruins",
			NPCIDs:     []string{"npc_cassian_red"},
			CreatureWeights: []worldpack.WeightedCreature{
				{CreatureID: "creature_skeleton", Weight: 0.5},
				{CreatureID: "creature_zombie", Weight: 0.3},
			},
			EncounterTableID: "encounter_ruins_night",
		},
		{
			LocationID:       "undercrypt_entrance",
			NPCIDs:           []string{"npc_mortis"},
			ItemIDs:          []string{"item_undercrypt_key"},
			EncounterTableID: "encounter_dungeon_entry",
		},
		{
			LocationID: "undercrypt_chamber_of_bones",
			CreatureWeights: []worldpack.WeightedCreature{
				{CreatureID: "creature_ghoul", Weight: 0.6},
				{CreatureID: "creature_skeleton", Weight: 0.3},
			},
			EncounterTableID: "encounter_dungeon_deep",
		},
		{
			LocationID: "coast_shipwreck_cove",
			CreatureWeights: []worldpack.WeightedCreature{
				{CreatureID: "creature_giant_rat", Weight: 0.8},
				{CreatureID: "creature_bandit", Weight: 0.2},
			},
			EncounterTableID: "encounter_coast",
		},
		{
			LocationID: "sunlit_lighthouse",
			NPCIDs:     []string{"npc_old_pel"},
			ItemIDs:    []string{"item_lighthouse_lens"},
		},
		{
			LocationID: "ironspine_pass",
			CreatureWeights: []worldpack.WeightedCreature{
				{CreatureID: "creature_hobgoblin", Weight: 0.4},
				{CreatureID: "creature_ogre", Weight: 0.2},
			},
			EncounterTableID: "encounter_mountain_road",
		},
		{
			LocationID:       "marches_old_bridge",
			EncounterTableID: "encounter_road_day",
		},
	}
	for _, lc := range contents {
		worldpack.AddLocationContents(p, lc)
	}
}

func buildEncounterTables(p *worldpack.Pack) {
	tables := []worldpack.EncounterTable{
		{
			ID: "encounter_whisperwood", Name: "Whisperwood Forest", Context: "forest", Biome: "forest", Dice: "d12",
			Description: "Daytime forest travel between King's Road and groves.",
			Rows: []worldpack.EncounterTableRow{
				{Roll: "1-2", Result: "No encounter; unsettling silence.", Notes: "Foreshadow fey attention."},
				{Roll: "3-4", Result: "1d4 wolves stalk the party.", CreatureIDs: []string{"creature_wolf"}, Quantity: "1d4"},
				{Roll: "5-6", Result: "2d4 goblins in ambush.", CreatureIDs: []string{"creature_goblin"}, Quantity: "2d4"},
				{Roll: "7-8", Result: "1 bugbear with 2 goblins.", CreatureIDs: []string{"creature_bugbear", "creature_goblin"}, Quantity: "1 + 2"},
				{Roll: "9-10", Result: "Giant spider webs block path.", CreatureIDs: []string{"creature_giant_spider"}, Quantity: "1d2"},
				{Roll: "11", Result: "Nim Willow offers guidance.", Notes: "Social; not combat."},
				{Roll: "12", Result: "Fey lights lead off-trail (optional side quest)."},
			},
		},
		{
			ID: "encounter_road_day", Name: "King's Road (Day)", Context: "road", Biome: "road", Dice: "d10",
			Rows: []worldpack.EncounterTableRow{
				{Roll: "1-3", Result: "Uneventful travel."},
				{Roll: "4-5", Result: "Merchant caravan (trade rumors)."},
				{Roll: "6-7", Result: "1d6 bandits demand toll.", CreatureIDs: []string{"creature_bandit"}, Quantity: "1d6"},
				{Roll: "8", Result: "Wolf pair crosses road.", CreatureIDs: []string{"creature_wolf"}, Quantity: "2"},
				{Roll: "9", Result: "Broken cart; Perception DC 13 for ambush.", CreatureIDs: []string{"creature_bandit"}, Quantity: "1d4"},
				{Roll: "10", Result: "Order paladin escort shares Undercrypt warning."},
			},
		},
		{
			ID: "encounter_urban_night", Name: "Millhaven Urban Night", Context: "urban_night", Biome: "urban", DistrictID: "undercroft", Dice: "d8",
			Rows: []worldpack.EncounterTableRow{
				{Roll: "1-2", Result: "Drunk commoners; no threat."},
				{Roll: "3-4", Result: "1d4 giant rats in trash heap.", CreatureIDs: []string{"creature_giant_rat"}, Quantity: "1d4"},
				{Roll: "5-6", Result: "Red Hand shake-down.", CreatureIDs: []string{"creature_bandit"}, Quantity: "1d4+1"},
				{Roll: "7", Result: "Guard patrol (Captain Thorne if alarm raised).", CreatureIDs: []string{"creature_guard"}, Quantity: "2d4"},
				{Roll: "8", Result: "Sable Quinn offers a job.", Notes: "Social encounter."},
			},
		},
		{
			ID: "encounter_dungeon_entry", Name: "Undercrypt Entry", Context: "dungeon", DungeonDepth: 1, Dice: "d6",
			Rows: []worldpack.EncounterTableRow{
				{Roll: "1", Result: "Cold draft; no encounter."},
				{Roll: "2-3", Result: "1d6 zombies shamble from rubble.", CreatureIDs: []string{"creature_zombie"}, Quantity: "1d6"},
				{Roll: "4-5", Result: "1d4 skeletons with shortbows.", CreatureIDs: []string{"creature_skeleton"}, Quantity: "1d4"},
				{Roll: "6", Result: "Brother Mortis preaching to converts.", Notes: "Roleplay or combat."},
			},
		},
		{
			ID: "encounter_dungeon_deep", Name: "Undercrypt Deep", Context: "dungeon", DungeonDepth: 3, Dice: "d8",
			Rows: []worldpack.EncounterTableRow{
				{Roll: "1-2", Result: "1d4 ghouls feeding.", CreatureIDs: []string{"creature_ghoul"}, Quantity: "1d4"},
				{Roll: "3-4", Result: "Giant spider nest.", CreatureIDs: []string{"creature_giant_spider"}, Quantity: "1"},
				{Roll: "5-6", Result: "2d6 skeletons.", CreatureIDs: []string{"creature_skeleton"}, Quantity: "2d6"},
				{Roll: "7", Result: "Crypt trap: DC 14 DEX save or 2d10 piercing."},
				{Roll: "8", Result: "Hollow Crown cultists (use cultist stats or bandits).", CreatureIDs: []string{"creature_bandit"}, Quantity: "2d4"},
			},
		},
		{
			ID: "encounter_coast", Name: "Sunlit Coast", Context: "coast", Biome: "coast", Dice: "d8",
			Rows: []worldpack.EncounterTableRow{
				{Roll: "1-3", Result: "Seagulls and tide; calm."},
				{Roll: "4", Result: "Smugglers unloading (Persuasion or fight).", CreatureIDs: []string{"creature_bandit"}, Quantity: "1d6"},
				{Roll: "5-6", Result: "Swarm of giant rats in wreck.", CreatureIDs: []string{"creature_giant_rat"}, Quantity: "2d6"},
				{Roll: "7", Result: "Old Pel signals from lighthouse."},
				{Roll: "8", Result: "Hidden cache: 2d6 gp and a healing potion.", Notes: "item_healing_potion"},
			},
		},
		{
			ID: "encounter_mountain_road", Name: "Ironspine Pass", Context: "mountain", Biome: "mountain", Dice: "d10",
			Rows: []worldpack.EncounterTableRow{
				{Roll: "1-3", Result: "Wind and goats; no encounter."},
				{Roll: "4-5", Result: "2d4 kobold slingers.", CreatureIDs: []string{"creature_kobold"}, Quantity: "2d4"},
				{Roll: "6-7", Result: "Hobgoblin squad.", CreatureIDs: []string{"creature_hobgoblin"}, Quantity: "4"},
				{Roll: "8", Result: "1 ogre blocks bridge.", CreatureIDs: []string{"creature_ogre"}, Quantity: "1"},
				{Roll: "9", Result: "Orc raiders.", CreatureIDs: []string{"creature_orc"}, Quantity: "2d4"},
				{Roll: "10", Result: "Ironhold patrol assists if party is flagged friendly.", CreatureIDs: []string{"creature_guard"}, Quantity: "4"},
			},
		},
		{
			ID: "encounter_ruins_night", Name: "Caer Mor at Night", Context: "ruins", Biome: "ruins", Dice: "d6",
			Rows: []worldpack.EncounterTableRow{
				{Roll: "1", Result: "Wind through empty halls."},
				{Roll: "2-3", Result: "1d6 skeletons rise.", CreatureIDs: []string{"creature_skeleton"}, Quantity: "1d6"},
				{Roll: "4-5", Result: "1d4 zombies with Undead Fortitude.", CreatureIDs: []string{"creature_zombie"}, Quantity: "1d4"},
				{Roll: "6", Result: "Cassian's war camp (major encounter).", CreatureIDs: []string{"creature_bandit", "creature_orc"}, Quantity: "2d6 + 1d4"},
			},
		},
		{
			ID: "encounter_millhaven_docks_day", Name: "Millhaven Docks (Day)", Context: "urban", Biome: "coast", Dice: "d6",
			Rows: []worldpack.EncounterTableRow{
				{Roll: "1-2", Result: "Busy labor; no threat."},
				{Roll: "3-4", Result: "Dockside argument escalates.", CreatureIDs: []string{"creature_commoner"}, Quantity: "2"},
				{Roll: "5", Result: "Smuggler chase through warehouses.", CreatureIDs: []string{"creature_bandit"}, Quantity: "1d4"},
				{Roll: "6", Result: "Fenn Reed offers passage intel.", Notes: "Social."},
			},
		},
	}
	for _, t := range tables {
		worldpack.AddEncounterTable(p, t)
	}
}
