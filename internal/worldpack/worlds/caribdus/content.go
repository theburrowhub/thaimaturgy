package caribdus

import (
	"github.com/theburrowhub/thaimaturgy/internal/worldpack"
)

func buildFactions(p *worldpack.Pack) {
	worldpack.AddFaction(p, "crown_armada", "Crown Armada",
		"Colonial navy and marines enforcing tariffs, charters, and hangings.",
		"Control every harbor; crush pirate republics; monopolize powder and ship timber.")
	worldpack.AddFaction(p, "free_corsair_council", "Free Corsair Council",
		"Loose confederation of pirate captains voting at Tortuga-style moots.",
		"Keep ports free of Crown law; split spoils fairly; hang traitors from the yardarm.")
	worldpack.AddFaction(p, "coven_tides", "Coven of Tides",
		"Sea-witch sisterhood binding storms, curses, and tidal omens.",
		"Protect Ghost Shoals; punish oath-breakers; recover Deluge relics from the trench.")
}

func buildLore(p *worldpack.Pack) {
	worldpack.AddLore(p, "lore_great_deluge", "The Great Deluge",
		"Three centuries ago the sea rose in a single season, swallowing empires and leaving Caribdus a reef-strewn graveyard. Sailors say the trench still belches the bones of drowned cities.",
		"central_archipelago", "history", "deluge")
	worldpack.AddLore(p, "lore_broken_anchors", "Treaty of Broken Anchors",
		"A fragile truce between Crown Armada and the Corsair Council: no firing in designated anchorages, exchange of prisoners, and shared charts through neutral Perla Azul. Both sides cheat.",
		"central_archipelago", "politics", "treaty")
	worldpack.AddLore(p, "lore_black_pearl_curse", "Curse of the Black Pearl",
		"A jet pearl taken from the trench is said to weigh down a captain's soul. Ships carrying one never outrun the same storm twice.",
		"deep_trench", "curse", "treasure")
	worldpack.AddLore(p, "lore_shoal_bells", "Bells of the Ghost Shoals",
		"Drowned sailors ring wrecks at dusk. Hearing the bells thrice marks you for the sea hags unless a witch breaks the omen with salt and blood.",
		"ghost_shoals", "supernatural", "undead")
}

func buildItems(p *worldpack.Pack) {
	items := []worldpack.WorldItem{
		{ID: "item_cutlass", Name: "Cutlass", Kind: "weapon", Rarity: "common", Description: "Curved naval saber.", Mechanics: "Str+d8; −1 Parry if paired with pistol.", ValueGP: 15, Tags: []string{"martial", "melee"}},
		{ID: "item_flintlock", Name: "Flintlock Pistol", Kind: "weapon", Rarity: "common", Description: "Single-shot naval sidearm.", Mechanics: "Range 5/10/20; 2d6+1; reload 2 actions.", ValueGP: 50, Tags: []string{"firearm", "ranged"}},
		{ID: "item_musket", Name: "Naval Musket", Kind: "weapon", Rarity: "common", Description: "Long arm for marines and hunters.", Mechanics: "Range 24/48/96; 2d8; reload 2 actions.", ValueGP: 75, Tags: []string{"firearm", "ranged"}},
		{ID: "item_grog", Name: "Grog Ration", Kind: "gear", Rarity: "common", Description: "Watered rum ration.", Mechanics: "+1 Spirit recovery; −1 Agility until sober.", ValueGP: 1, Tags: []string{"consumable"}},
		{ID: "item_rum_fine", Name: "Fine Aged Rum", Kind: "gear", Rarity: "uncommon", Description: "Caribdus dark rum.", Mechanics: "Bribe or Persuade +1 once; hangover −1 Vigor next day.", ValueGP: 10, Tags: []string{"consumable", "trade"}},
		{ID: "item_compass", Name: "Brass Compass", Kind: "gear", Rarity: "common", Description: "Reliable unless near Ghost Shoals.", Mechanics: "Navigation +1; fails in ghost_shoals without witch ward.", ValueGP: 25, Tags: []string{"navigation"}},
		{ID: "item_spyglass", Name: "Spyglass", Kind: "gear", Rarity: "common", Description: "Naval observation glass.", Mechanics: "Notice +2 at range; identify flags at 1 mile.", ValueGP: 20, Tags: []string{"navigation"}},
		{ID: "item_diving_helmet", Name: "Brass Diving Helmet", Kind: "gear", Rarity: "uncommon", Description: "Pump-fed helm for shallow trench work.", Mechanics: "Survive 30 min at depth; Athletics −1.", ValueGP: 150, Tags: []string{"diving"}},
		{ID: "item_powder_horn", Name: "Powder Horn", Kind: "gear", Rarity: "common", Description: "Watered-proofed black powder store.", Mechanics: "20 shots; wet ruins half.", ValueGP: 8, Tags: []string{"firearm"}},
		{ID: "item_grappling_hook", Name: "Grappling Hook & Line", Kind: "gear", Rarity: "common", Description: "Boarding gear.", Mechanics: "Athletics +1 to climb rigging or walls.", ValueGP: 5, Tags: []string{"adventuring"}},
		{ID: "item_healing_rum", Name: "Medic's Rum & Salve", Kind: "potion", Rarity: "common", Description: "Rough field medicine.", Mechanics: "Heal 1d6+1 once; −1 Agility 1 hour.", ValueGP: 15, Tags: []string{"consumable"}},
		{ID: "item_cursed_pearl", Name: "Black Pearl of the Trench", Kind: "magic", Rarity: "rare", Description: "Jet pearl warm to the touch.", Mechanics: "Worth 500 gp; owner draws storms (GM discretion).", ValueGP: 500, Tags: []string{"curse", "treasure"}},
		{ID: "item_witch_charm", Name: "Sea-Witch Salt Charm", Kind: "magic", Rarity: "uncommon", Description: "Knotted kelp pouch.", Mechanics: "Ignore one curse or ghost-shoals compulsion.", ValueGP: 75, Tags: []string{"magic", "sea-witch"}},
		{ID: "item_silver_cutlass", Name: "Silver-Edged Cutlass", Kind: "weapon", Rarity: "uncommon", Description: "Silver inlaid blade.", Mechanics: "Str+d8+1; silver damage vs undead.", ValueGP: 120, Tags: []string{"magic", "silver"}},
		{ID: "item_sailor_rope", Name: "Tarred Sailor's Rope (50 ft.)", Kind: "gear", Rarity: "common", Description: "Standard ship rope.", Mechanics: "Standard adventuring gear.", ValueGP: 2, Tags: []string{"adventuring"}},
		{ID: "item_chart_shoals", Name: "Smuggler's Shoal Chart", Kind: "gear", Rarity: "uncommon", Description: "Hidden reef passages.", Mechanics: "Navigation +2 in central_archipelago; illegal in Crown ports.", ValueGP: 40, Tags: []string{"navigation", "smuggling"}, LocationIDs: []string{"cala_contrabandistas"}},
	}
	for _, it := range items {
		worldpack.AddItem(p, it)
	}
}
