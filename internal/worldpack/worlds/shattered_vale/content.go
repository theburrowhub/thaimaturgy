package shattered_vale

import (
	"github.com/theburrowhub/thaimaturgy/internal/worldpack"
)

func buildFactions(p *worldpack.Pack) {
	worldpack.AddFaction(p, "merchants_league", "Merchants' League",
		"Cartel of trade guilds controlling river tariffs and warehouse licenses.",
		"Maximize profit, monopolize grain routes, keep the vale politically fragmented.")
	worldpack.AddFaction(p, "order_of_dawn", "Order of the Dawn",
		"Militant faith dedicated to holding back undeath and abyssal corruption from the Shattering.",
		"Seal Undercrypt breaches, support temples, recruit paladins and clerics.")
	worldpack.AddFaction(p, "red_hand", "Red Hand Bandits",
		"Loose confederation of brigands marked by crimson hand graffiti.",
		"Extort caravans, raid weak settlements, eventually control Thornwall pass.")
}

func buildLore(p *worldpack.Pack) {
	worldpack.AddLore(p, "lore_shattering", "The Shattering",
		"Five years ago a failed archmage ritual cracked the vale's ley lines. Cities survived; the spaces between them turned hostile. Undead rise faster near fault lines.",
		"", "history", "cataclysm")
	worldpack.AddLore(p, "lore_silverrun", "The Silverrun River",
		"Major trade artery from Ironhold ore barges to Millhaven's sea gates. River pirates are rare but smugglers common.",
		"sunlit_coast", "trade", "geography")
	worldpack.AddLore(p, "lore_undercrypt", "Secrets of the Undercrypt",
		"The necropolis predates the empire. Cult of the Hollow Crown seeks a lich regent in the deepest vaults.",
		"undercrypt", "undead", "plot")
	worldpack.AddLore(p, "lore_whisperwood", "Whispers in the Wood",
		"Shepherds report trees repeating their secrets. Druids say the forest listens to the Shattering and learns fear.",
		"whisperwood", "fey", "horror")
	worldpack.AddLore(p, "lore_red_hand", "Origins of the Red Hand",
		"Bandit lord Cassian united outcast soldiers after the Shattering. The Hand taxes the moor road and fences stolen League goods in Millhaven's Undercroft.",
		"northern_marches", "bandits", "politics")
}

func buildItems(p *worldpack.Pack) {
	items := []worldpack.WorldItem{
		{ID: "item_longsword", Name: "Longsword", Kind: "weapon", Rarity: "common", Description: "Versatile steel blade.", Mechanics: "1d8 slashing (1d10 two-handed).", ValueGP: 15, Tags: []string{"martial", "melee"}},
		{ID: "item_shortsword", Name: "Shortsword", Kind: "weapon", Rarity: "common", Description: "Light blade favored by scouts.", Mechanics: "1d6 piercing, finesse, light.", ValueGP: 10, Tags: []string{"martial", "finesse"}},
		{ID: "item_greatsword", Name: "Greatsword", Kind: "weapon", Rarity: "common", Description: "Heavy two-handed sword.", Mechanics: "2d6 slashing, heavy, two-handed.", ValueGP: 50, Tags: []string{"martial", "heavy"}},
		{ID: "item_light_crossbow", Name: "Light Crossbow", Kind: "weapon", Rarity: "common", Description: "Simple ranged weapon.", Mechanics: "1d8 piercing, range 80/320, loading.", ValueGP: 25, Tags: []string{"ranged"}},
		{ID: "item_dagger", Name: "Dagger", Kind: "weapon", Rarity: "common", Description: "Utility blade.", Mechanics: "1d4 piercing, finesse, light, thrown 20/60.", ValueGP: 2, Tags: []string{"simple", "finesse"}},
		{ID: "item_spear", Name: "Spear", Kind: "weapon", Rarity: "common", Description: "Guard issue polearm.", Mechanics: "1d6 piercing (1d8 two-handed); thrown 20/60.", ValueGP: 1, Tags: []string{"simple"}},
		{ID: "item_leather_armor", Name: "Leather Armor", Kind: "armor", Rarity: "common", Description: "Supple hide armor.", Mechanics: "AC 11 + DEX.", ValueGP: 10, Tags: []string{"light"}},
		{ID: "item_chain_shirt", Name: "Chain Shirt", Kind: "armor", Rarity: "common", Description: "Interlocking rings.", Mechanics: "AC 13 + DEX (max 2).", ValueGP: 50, Tags: []string{"medium"}},
		{ID: "item_scale_mail", Name: "Scale Mail", Kind: "armor", Rarity: "common", Description: "Ironhold garrison issue.", Mechanics: "AC 14 + DEX (max 2); Stealth disadvantage.", ValueGP: 50, Tags: []string{"medium"}},
		{ID: "item_shield", Name: "Shield", Kind: "armor", Rarity: "common", Description: "Wooden shield with iron boss.", Mechanics: "+2 AC.", ValueGP: 10, Tags: []string{"shield"}},
		{ID: "item_healing_potion", Name: "Potion of Healing", Kind: "potion", Rarity: "common", Description: "Red glimmering draught.", Mechanics: "Regain 2d4+2 HP when consumed.", ValueGP: 50, Tags: []string{"consumable", "magic"}},
		{ID: "item_antitoxin", Name: "Antitoxin", Kind: "gear", Rarity: "common", Description: "Herbal neutralizer.", Mechanics: "Advantage on saves vs poison for 1 hour.", ValueGP: 50, Tags: []string{"consumable"}},
		{ID: "item_healers_kit", Name: "Healer's Kit", Kind: "gear", Rarity: "common", Description: "Bandages, salves, splints.", Mechanics: "Stabilize dying creature without Medicine check; 10 uses.", ValueGP: 5, Tags: []string{"tool"}},
		{ID: "item_thieves_tools", Name: "Thieves' Tools", Kind: "gear", Rarity: "common", Description: "Lockpicks and pry bars.", Mechanics: "Required for lockpicking; proficiency adds PB.", ValueGP: 25, Tags: []string{"tool"}},
		{ID: "item_rope", Name: "Hempen Rope (50 ft.)", Kind: "gear", Rarity: "common", Description: "Sturdy climbing rope.", Mechanics: "Standard adventuring gear.", ValueGP: 1, Tags: []string{"adventuring"}},
		{ID: "item_torch", Name: "Torch", Kind: "gear", Rarity: "common", Description: "Pine-resin torch.", Mechanics: "Bright light 20 ft., dim 20 ft.; 1 hour burn.", ValueGP: 1, Tags: []string{"light"}},
		{ID: "item_rations", Name: "Rations (1 day)", Kind: "gear", Rarity: "common", Description: "Traveler's dry provisions.", Mechanics: "One day of food.", ValueGP: 5, Tags: []string{"consumable"}, LocationIDs: []string{"millhaven_market"}},
		{ID: "item_climbing_kit", Name: "Climbing Kit", Kind: "gear", Rarity: "common", Description: "Pitons, gloves, harness.", Mechanics: "Advantage on Athletics to climb.", ValueGP: 25, Tags: []string{"adventuring"}},
		{ID: "item_smith_hammer", Name: "Stoneforge Warhammer", Kind: "weapon", Rarity: "uncommon", Description: "Helga Stone's balanced hammer.", Mechanics: "+1 warhammer (1d8+1 bludgeoning).", ValueGP: 200, Tags: []string{"magic", "+1"}, LocationIDs: []string{"ironhold_smithy"}},
		{ID: "item_dawn_amulet", Name: "Amulet of the Dawn", Kind: "magic", Rarity: "uncommon", Description: "Order holy symbol warm to the touch.", Mechanics: "Once/day cast bless (self only) as an action.", ValueGP: 300, Tags: []string{"magic", "holy"}, LocationIDs: []string{"temple_of_dawn"}},
		{ID: "item_red_hand_mask", Name: "Red Hand Mask", Kind: "gear", Rarity: "common", Description: "Crimson cloth mask, bandit trophy.", Mechanics: "Disadvantage on Persuasion with law-abiding NPCs if worn openly.", ValueGP: 0, Tags: []string{"criminal"}},
		{ID: "item_moonpetal_herb", Name: "Moonpetal Salve", Kind: "potion", Rarity: "common", Description: "Nim Willow's herbal salve.", Mechanics: "Cure one level of exhaustion after short rest (once/day).", ValueGP: 25, Tags: []string{"herbal"}, LocationIDs: []string{"whisperwood_grove"}},
		{ID: "item_silver_dagger", Name: "Silvered Dagger", Kind: "weapon", Rarity: "uncommon", Description: "Silver-coated blade for lycanthropes and certain undead.", Mechanics: "1d4+1 piercing; counts as silver for resistances.", ValueGP: 100, Tags: []string{"silver", "finesse"}},
		{ID: "item_bag_of_trinkets", Name: "Dockside Curio Bag", Kind: "gear", Rarity: "common", Description: "Assorted sea charms and odd coins.", Mechanics: "Flavor loot; single trinket worth 1d6 sp.", ValueGP: 3, LocationIDs: []string{"river_docks"}},
		{ID: "item_undercrypt_key", Name: "Crypt Iron Key", Kind: "key", Rarity: "rare", Description: "Heavy key etched with crown sigil.", Mechanics: "Opens sealed Undercrypt vault (DM plot gate).", ValueGP: 0, Tags: []string{"quest"}},
		{ID: "item_lighthouse_lens", Name: "Focusing Lens Shard", Kind: "magic", Rarity: "uncommon", Description: "Fractured lighthouse lens that catches starlight.", Mechanics: "Once/day cast light on an object for 1 hour.", ValueGP: 75, LocationIDs: []string{"sunlit_lighthouse"}},
	}
	for _, it := range items {
		worldpack.AddItem(p, it)
	}
}
