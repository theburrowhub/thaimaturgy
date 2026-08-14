package mistfall_coast

import (
	"github.com/theburrowhub/thaimaturgy/internal/worldpack"
)

func buildFactions(p *worldpack.Pack) {
	worldpack.AddFaction(p, "harbor_board", "Harrowport Harbor Board", "Tariff-setting oligarchy of shipping magnates.", "Maximize revenue and suppress scandal.")
	worldpack.AddFaction(p, "mist_runners", "Mist Runners", "Smuggler confederation using fog and bribed pilots.", "Free trade and no Crown tariffs.")
	worldpack.AddFaction(p, "drowned_lantern", "Drowned Lantern", "Cult venerating consciousness in the fog sea.", "Sink a lighthouse beacon during spring equinox.")
	worldpack.AddFaction(p, "salt_ledger", "Salt Ledger Press", "Investigative reform newspaper.", "Expose corruption and missing fishers.")
}

func buildLore(p *worldpack.Pack) {
	worldpack.AddLore(p, "lore_deluge_fog", "The Persistent Mist", "Sailors say the fog thickened after the 1887 Deluge Night when eleven boats vanished in clear weather.", "harrow_bay", "history")
	worldpack.AddLore(p, "lore_mine_seal", "Shaft Seven", "Brackenford sealed Mine Shaft 7 after miners reported singing from flooded tunnels.", "blackwood_hills", "horror")
	worldpack.AddLore(p, "lore_black_lantern", "Black Lantern", "The wreck appears on flats each decade; keepers who investigate do not return unchanged.", "salt_flats", "mystery")
}

func buildItems(p *worldpack.Pack) {
	items := []worldpack.WorldItem{
		{ID: "item_webley", Name: "Webley Revolver", Kind: "weapon", Rarity: "common", Description: ".455 service revolver.", Mechanics: "Handgun skill; 1d10+2 damage.", ValueGP: 35, Tags: []string{"firearm"}},
		{ID: "item_fog_lamp", Name: "Brass Fog Lamp", Kind: "gear", Rarity: "common", Description: "Hand lamp with green glass.", Mechanics: "Navigate fog without penalty for 1 hour.", ValueGP: 8, Tags: []string{"exploration"}},
		{ID: "item_sanity_tonic", Name: "Physician's Nerve Tonic", Kind: "consumable", Rarity: "uncommon", Description: "Bitter laudanum blend.", Mechanics: "Recover 1d3 Sanity; CON roll or drowsy.", ValueGP: 12, Tags: []string{"medicine"}},
		{ID: "item_lantern_badge", Name: "Lighthouse Keeper Badge", Kind: "gear", Rarity: "uncommon", Description: "Brass badge with lantern crest.", Mechanics: "Access service ladders; cultists recognize symbol.", ValueGP: 0, Tags: []string{"key"}},
		{ID: "item_salt_ledger_press", Name: "Salt Ledger Extra", Kind: "gear", Rarity: "common", Description: "Damning article draft.", Mechanics: "Persuade +1 vs Harbor Board once.", ValueGP: 0, Tags: []string{"plot"}},
		{ID: "item_drowned_idol", Name: "Jade Drowned Idol", Kind: "magic", Rarity: "rare", Description: "Warm figurine from tidal shrine.", Mechanics: "POW roll or lose 1d4 Sanity on first touch.", ValueGP: 200, Tags: []string{"cult", "cursed"}},
	}
	for _, it := range items {
		worldpack.AddItem(p, it)
	}
}
