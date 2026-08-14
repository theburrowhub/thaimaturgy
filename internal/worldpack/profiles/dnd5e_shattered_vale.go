package profiles

import (
	"strings"

	"github.com/theburrowhub/thaimaturgy/internal/domain"
	"github.com/theburrowhub/thaimaturgy/internal/srd"
	"github.com/theburrowhub/thaimaturgy/internal/worldpack"
)

// DnD5eShatteredVale returns a rich generic fantasy world pack for D&D 5e.
func DnD5eShatteredVale() *worldpack.Pack {
	p := NewBaseWorld("dnd5e_shattered_vale", "The Shattered Vale", "The Shattered Vale", "dnd5e")
	SetSettingTone(p,
		"Late medieval fantasy, five years after the Shattering",
		"Heroic with creeping dread; trade hubs bustle while wilderness grows feral",
		"A river-valley region fractured by a magical cataclysm. City-states cling to roads and rivers while monsters reclaim the wilds.",
		"fantasy", "riverlands", "sandbox",
	)

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

func buildRegions(p *worldpack.Pack) {
	AddRegion(p, "northern_marches", "Northern Marches",
		"Rolling moorland and broken keeps north of the vale. Bandits and undead haunt the old imperial roads.",
		"grassland", "cold", "ruins", "bandits")
	p.Regions[0].CityIDs = []string{"thornwall"}
	p.Regions[0].TravelNotes = "Two days from Millhaven by road; winter snows can close the pass for weeks."
	p.Regions[0].MapID = "map_northern_marches"

	AddRegion(p, "sunlit_coast", "Sunlit Coast",
		"Cliff-lined shores and fishing villages where smugglers and sahuagin rumors mix with salt spray.",
		"coastal", "trade", "smuggling")
	p.Regions[1].CityIDs = []string{"millhaven"}
	p.Regions[1].TravelNotes = "Coastal road is well patrolled near Millhaven; ship travel to Ironhold takes half a day."
	p.Regions[1].MapID = "map_sunlit_coast"

	AddRegion(p, "whisperwood", "Whisperwood",
		"An ancient forest whose trees murmur at dusk. Fey trails and goblin warrens lurk beneath the canopy.",
		"forest", "fey", "goblin")
	p.Regions[2].TravelNotes = "Travelers should not leave the King's Road after dark."
	p.Regions[2].MapID = "map_whisperwood"

	AddRegion(p, "ironspine_mountains", "Ironspine Mountains",
		"Jagged peaks rich in ore. Hobgoblin legions drill in hidden valleys; ogres block high passes.",
		"mountain", "mining", "hobgoblin")
	p.Regions[3].CityIDs = []string{"ironhold"}
	p.Regions[3].TravelNotes = "Mountain passes require Survival DC 12 in winter; avalanches are common."
	p.Regions[3].MapID = "map_ironspine"

	AddRegion(p, "undercrypt", "Undercrypt",
		"A buried necropolis opened by the Shattering. Cultists and undead swell its halls.",
		"underground", "undead", "dungeon")
	p.Regions[4].TravelNotes = "No safe overland route; entrances are scattered sinkholes and collapsed temples."
	p.Regions[4].MapID = "map_undercrypt"
}

func buildMaps(p *worldpack.Pack) {
	AddMap(p, "map_northern_marches", "Northern Marches Overview", "regional",
		"maps/northern_marches.png", "Moor roads, ruined towers, Thornwall marked at the frontier.", "1 hex = 6 miles")
	AddMap(p, "map_sunlit_coast", "Sunlit Coast Chart", "regional",
		"maps/sunlit_coast.png", "River delta, Millhaven harbor, cliff paths.", "1 hex = 4 miles")
	AddMap(p, "map_whisperwood", "Whisperwood Trails", "regional",
		"maps/whisperwood.png", "King's Road, grove shrines, goblin territory hatched in green.", "1 hex = 4 miles")
	AddMap(p, "map_ironspine", "Ironspine Passes", "regional",
		"maps/ironspine.png", "Fortress Ironhold, mining camps, hobgoblin markers.", "1 hex = 5 miles")
	AddMap(p, "map_undercrypt", "Undercrypt Levels", "dungeon",
		"maps/undercrypt.png", "Collapsed temple entrance and three known sub-levels.", "1 square = 10 ft")
	AddMap(p, "map_millhaven", "Millhaven City Map", "city",
		"maps/millhaven.png", "Districts: Harbor, Market, Temple Hill, Garrison, Undercroft.", "1 square = 100 ft")
	AddMap(p, "map_ironhold", "Ironhold Fortress", "city",
		"maps/ironhold.png", "Keep, smith quarter, training yard, sally ports.", "1 square = 50 ft")
}

func buildCitiesAndLocations(p *worldpack.Pack) {
	// --- Millhaven ---
	millhavenDistricts := []worldpack.District{
		AddDistrict("harbor", "Harbor District", "Salt-stained docks and warehouses.", nil, "trade", "docks"),
		AddDistrict("market", "Market District", "Bustling bazaar and guild halls.", nil, "trade", "commerce"),
		AddDistrict("temple_hill", "Temple Hill", "White stone temples overlooking the river.", nil, "holy"),
		AddDistrict("garrison", "Garrison Quarter", "Barracks and the town wall's eastern gate.", nil, "military"),
		AddDistrict("undercroft", "Undercroft", "Narrow alleys beneath Temple Hill; thieves and fences.", nil, "criminal"),
	}
	AddCity(p, "millhaven", "Millhaven", "sunlit_coast",
		"River-port trade hub of twenty thousand souls. The Merchants' League holds sway, but the Order of the Dawn keeps the peace.",
		millhavenDistricts, "trade", "port", "hub")
	p.Cities[0].Population = "~20,000"
	p.Cities[0].Government = "Merchant Council chaired by Mayor Eldric Vane"
	p.Cities[0].MapID = "map_millhaven"
	LinkCityToRegion(p, "sunlit_coast", "millhaven")

	// Millhaven locations
	addLoc(p, worldpack.Location{
		ID: "millhaven_market", Name: "Grand Market Square", Kind: "market",
		CityID: "millhaven", DistrictID: "market", RegionID: "sunlit_coast",
		Description: "Canvas awnings shade stalls selling spice, steel, and spell components. Town criers announce League tariffs at noon.",
		ReadAloud:   "The market roars with haggling voices. Spice smoke and forge heat mingle above cobblestones worn smooth by centuries of trade.",
		DMNotes:     "Pickpockets (commoner stats) operate in pairs; Merchant Prince Alden Cross maintains a private booth on the east side.",
		Tags:        []string{"market", "social", "shopping"},
		Connections: []string{"the_gilded_anchor", "millhaven_town_hall"},
	})
	addLoc(p, worldpack.Location{
		ID: "the_gilded_anchor", Name: "The Gilded Anchor", Kind: "tavern",
		CityID: "millhaven", DistrictID: "market", RegionID: "sunlit_coast",
		Description: "Three-story inn favored by river captains. Rooms 5 sp/night; stew 3 cp; ale 4 cp.",
		ReadAloud:   "A gilt ship's anchor hangs above the door. Inside, a hearth fire fights the river damp while a fiddle tunes up for the evening.",
		DMNotes:     "Tomas Gull knows every rumor on the docks. Hidden cellar connects to cutpurse alley for smugglers.",
		Tags:        []string{"tavern", "rest", "rumors"},
		Connections: []string{"millhaven_market", "river_docks"},
	})
	addLoc(p, worldpack.Location{
		ID: "temple_of_dawn", Name: "Temple of the Dawn", Kind: "temple",
		CityID: "millhaven", DistrictID: "temple_hill", RegionID: "sunlit_coast",
		Description: "Marble sanctuary of the Order of the Dawn. Healing available for donations; undead wards on the crypt door.",
		ReadAloud:   "Stained glass casts amber light across pews. Incense and quiet prayer fill the nave.",
		DMNotes:     "Priestess Lyra Dawn can raise concerns about Undercrypt activity. Relic vault DC 18 Thieves' Tools.",
		Tags:        []string{"temple", "healing", "holy"},
		Connections: []string{"millhaven_town_hall", "cutpurse_alley"},
	})
	addLoc(p, worldpack.Location{
		ID: "millhaven_barracks", Name: "Town Barracks", Kind: "barracks",
		CityID: "millhaven", DistrictID: "garrison", RegionID: "sunlit_coast",
		Description: "Stone barracks housing forty town guards. Armory issues weapons only with writ.",
		ReadAloud:   "Drill commands echo across the yard. Spears flash in practiced unison.",
		DMNotes:     "Captain Mira Thorne runs tight discipline. Cells in basement hold petty criminals.",
		Tags:        []string{"military", "law"},
		Connections: []string{"millhaven_market"},
	})
	addLoc(p, worldpack.Location{
		ID: "river_docks", Name: "River Docks", Kind: "docks",
		CityID: "millhaven", DistrictID: "harbor", RegionID: "sunlit_coast",
		Description: "Timber wharves along the Silverrun. Barges to Ironhold depart at dawn.",
		ReadAloud:   "Gulls wheel over moored barges. Dockhands shout as crane ropes strain under grain sacks.",
		DMNotes:     "Smugglers offload at night near warehouse 7. Dockmaster Fenn Reed takes bribes (5 gp) to look away.",
		Tags:        []string{"docks", "travel", "smuggling"},
		Connections: []string{"the_gilded_anchor", "sunlit_lighthouse"},
	})
	addLoc(p, worldpack.Location{
		ID: "cutpurse_alley", Name: "Cutpurse Alley", Kind: "street",
		CityID: "millhaven", DistrictID: "undercroft", RegionID: "sunlit_coast",
		Description: "Lamp-lit maze of lean-tos and bolt-holes. Red Hand graffiti marks territory.",
		ReadAloud:   "Wet stone narrows to shoulder width. Eyes gleam from shutter slits.",
		DMNotes:     "Sable Quinn runs a fence operation. Random encounter: 1d4 bandits or thugs shaking down a merchant.",
		Tags:        []string{"criminal", "urban", "danger"},
		Connections: []string{"temple_of_dawn", "millhaven_market"},
	})
	addLoc(p, worldpack.Location{
		ID: "millhaven_town_hall", Name: "Millhaven Town Hall", Kind: "civic",
		CityID: "millhaven", DistrictID: "market", RegionID: "sunlit_coast",
		Description: "Council chamber and mayor's office. Public petitions heard on tendays.",
		ReadAloud:   "A bell tower rises above civic banners. Clerks scratch ledgers behind iron-grille windows.",
		DMNotes:     "Mayor Eldric Vane balances League profit against Dawn moralism.",
		Tags:        []string{"civic", "quests"},
		Connections: []string{"millhaven_market", "temple_of_dawn"},
	})

	// --- Ironhold ---
	AddCity(p, "ironhold", "Ironhold", "ironspine_mountains",
		"Mountain fortress-city controlling ore routes. Warden Gareth rules with military pragmatism.",
		[]worldpack.District{
			AddDistrict("keep", "High Keep", "Citadel and officer quarters.", nil, "military"),
			AddDistrict("forge_quarter", "Forge Quarter", "Smelters and master smiths.", nil, "craft"),
		}, "fortress", "mining")
	p.Cities[1].Population = "~8,000"
	p.Cities[1].Government = "Military governorship under Warden Gareth"
	p.Cities[1].MapID = "map_ironhold"
	LinkCityToRegion(p, "ironspine_mountains", "ironhold")

	addLoc(p, worldpack.Location{
		ID: "ironhold_keep", Name: "Ironhold Keep", Kind: "keep",
		CityID: "ironhold", DistrictID: "keep", RegionID: "ironspine_mountains",
		Description: "Basalt keep carved into the cliff. Ballistae cover the pass.",
		ReadAloud:   "Black banners snap in the mountain wind. The gate groans open to reveal a courtyard of marching soldiers.",
		Tags:        []string{"military", "fortress"},
		Connections: []string{"ironhold_training_yard", "ironhold_smithy"},
	})
	addLoc(p, worldpack.Location{
		ID: "ironhold_smithy", Name: "Stoneforge Smithy", Kind: "shop",
		CityID: "ironhold", DistrictID: "forge_quarter", RegionID: "ironspine_mountains",
		Description: "Master smith Helga Stone crafts reliable arms for the garrison and visitors with coin.",
		ReadAloud:   "Hammer rhythm rings like a heartbeat. Coal heat washes over you as sparks fountain across the anvil.",
		Tags:        []string{"shop", "weapons", "armor"},
		Connections: []string{"ironhold_keep"},
	})
	addLoc(p, worldpack.Location{
		ID: "ironhold_training_yard", Name: "Training Yard", Kind: "yard",
		CityID: "ironhold", DistrictID: "keep", RegionID: "ironspine_mountains",
		Description: "Open yard for drills; recruits and mercenaries spar under sergeants' eyes.",
		ReadAloud:   "Wooden swords clack in synchronized forms. A drill sergeant barks corrections.",
		Tags:        []string{"military", "training"},
		Connections: []string{"ironhold_keep", "ironspine_pass"},
	})

	// --- Thornwall ---
	AddCity(p, "thornwall", "Thornwall", "northern_marches",
		"Palisade frontier town guarding the moor road. Last respectable stop before the wild marches.",
		[]worldpack.District{
			AddDistrict("gate", "Gate Town", "Inns and traders serving caravan traffic.", nil, "frontier"),
		}, "frontier", "caravan")
	p.Cities[2].Population = "~2,500"
	p.Cities[2].Government = "Frontier charter; Scout-Captain Jessa Marrow holds practical authority"
	LinkCityToRegion(p, "northern_marches", "thornwall")

	addLoc(p, worldpack.Location{
		ID: "thornwall_gatehouse", Name: "Thornwall Gatehouse", Kind: "gate",
		CityID: "thornwall", DistrictID: "gate", RegionID: "northern_marches",
		Description: "Double palisade with murder holes. Toll 1 sp per wagon.",
		ReadAloud:   "Thorn-studded logs frame the gate. Guards scan the moor road with wary eyes.",
		Tags:        []string{"frontier", "travel"},
		Connections: []string{"thornwall_saloon", "marches_old_bridge"},
	})
	addLoc(p, worldpack.Location{
		ID: "thornwall_saloon", Name: "The Broken Spear Saloon", Kind: "tavern",
		CityID: "thornwall", DistrictID: "gate", RegionID: "northern_marches",
		Description: "Rough frontier tavern. Bounties posted on the wall; Red Hand wanted posters among them.",
		ReadAloud:   "Sawdust floors soak up spilled ale. A fire crackles while traders swap road warnings.",
		Tags:        []string{"tavern", "frontier", "rumors"},
		Connections: []string{"thornwall_gatehouse"},
	})

	// --- Wilderness ---
	wildLocs := []worldpack.Location{
		{
			ID: "whisperwood_grove", Name: "Moonlit Grove", Kind: "wilderness",
			RegionID: "whisperwood",
			Description: "Circle of silver-barked trees; fey lights at new moon.",
			ReadAloud:   "The forest hushes. Pale mushrooms glow between roots like scattered coins.",
			Tags:        []string{"forest", "fey", "wilderness"},
			Connections: []string{"whisperwood_roadside_shrine", "marches_old_bridge"},
		},
		{
			ID: "whisperwood_roadside_shrine", Name: "King's Road Shrine", Kind: "shrine",
			RegionID: "whisperwood",
			Description: "Weathered saint statue; travelers leave coins for safe passage.",
			ReadAloud:   "A cracked saint watches the road. Offerings rust in a stone bowl.",
			Tags:        []string{"forest", "road", "holy"},
			Connections: []string{"whisperwood_grove", "marches_old_bridge"},
		},
		{
			ID: "ironspine_pass", Name: "Ironspine High Pass", Kind: "wilderness",
			RegionID: "ironspine_mountains",
			Description: "Wind-scoured pass; hobgoblin patrols at dusk.",
			ReadAloud:   "The path narrows along a cliff. Far below, a river cuts through fog.",
			Tags:        []string{"mountain", "travel", "danger"},
			Connections: []string{"ironhold_training_yard"},
		},
		{
			ID: "sunlit_coast_cliffs", Name: "Sunlit Cliffs", Kind: "wilderness",
			RegionID: "sunlit_coast",
			Description: "Chalk cliffs with sea caves; smuggler boats at low tide.",
			ReadAloud:   "Waves hammer stone. Gulls scream over tide pools glittering in afternoon sun.",
			Tags:        []string{"coast", "wilderness"},
			Connections: []string{"coast_shipwreck_cove", "sunlit_lighthouse"},
		},
		{
			ID: "sunlit_lighthouse", Name: "Pel's Lighthouse", Kind: "landmark",
			RegionID: "sunlit_coast", CityID: "millhaven", DistrictID: "harbor",
			Description: "Old lighthouse on the headland; keeper maintains the beacon for dock fees.",
			ReadAloud:   "A striped tower climbs from the headland. The beacon's glass catches the dying light.",
			Tags:        []string{"coast", "landmark"},
			Connections: []string{"river_docks", "sunlit_coast_cliffs"},
		},
		{
			ID: "northern_marches_ruins", Name: "Ruins of Caer Mor", Kind: "ruins",
			RegionID: "northern_marches",
			Description: "Collapsed imperial fort; skeletons and shadows at night.",
			ReadAloud:   "Broken towers claw at low clouds. Ivy masks arrow slits like closed eyes.",
			Tags:        []string{"ruins", "undead", "dungeon"},
			Connections: []string{"marches_old_bridge", "undercrypt_entrance"},
		},
		{
			ID: "marches_old_bridge", Name: "Old Moor Bridge", Kind: "wilderness",
			RegionID: "northern_marches",
			Description: "Stone bridge over a black river; toll collectors replaced by bandits some nights.",
			ReadAloud:   "Ancient arches span sluggish water. Carved wards have been chiseled away.",
			Tags:        []string{"road", "river", "bandits"},
			Connections: []string{"thornwall_gatehouse", "whisperwood_grove", "northern_marches_ruins"},
		},
		{
			ID: "coast_shipwreck_cove", Name: "Shipwreck Cove", Kind: "wilderness",
			RegionID: "sunlit_coast",
			Description: "Hidden cove with rotting hulls; giant rats and smuggler caches.",
			ReadAloud:   "Barnacle-crusted ribs of ships lean over black sand. The tide sucks at broken timbers.",
			Tags:        []string{"coast", "smuggling", "dungeon"},
			Connections: []string{"sunlit_coast_cliffs"},
		},
		{
			ID: "undercrypt_entrance", Name: "Collapsed Temple Entrance", Kind: "dungeon",
			RegionID: "undercrypt",
			Description: "Sinkhole revealing stairways into the necropolis. Cult graffiti marks the walls.",
			ReadAloud:   "Cold air exhales from broken flagstones. Torchlight does not reach the bottom.",
			Tags:        []string{"dungeon", "undead", "underground"},
			Connections: []string{"northern_marches_ruins", "undercrypt_chamber_of_bones"},
		},
		{
			ID: "undercrypt_chamber_of_bones", Name: "Chamber of Bones", Kind: "dungeon",
			RegionID: "undercrypt",
			Description: "Ossuary chamber; ghoul packs nest behind rib-cage arches.",
			ReadAloud:   "Skulls piled to the ceiling watch with empty sockets. Scraping echoes from side tunnels.",
			Tags:        []string{"dungeon", "undead", "underground"},
			Connections: []string{"undercrypt_entrance"},
		},
	}
	for _, loc := range wildLocs {
		addLoc(p, loc)
		LinkWildernessLocation(p, loc.RegionID, loc.ID)
	}

	// Link city locations
	for _, id := range []string{
		"millhaven_market", "the_gilded_anchor", "temple_of_dawn", "millhaven_barracks",
		"river_docks", "cutpurse_alley", "millhaven_town_hall",
	} {
		LinkLocationToCity(p, "millhaven", districtForMillhaven(id), id)
	}
	for _, id := range []string{"ironhold_keep", "ironhold_smithy", "ironhold_training_yard"} {
		LinkLocationToCity(p, "ironhold", districtForIronhold(id), id)
	}
	for _, id := range []string{"thornwall_gatehouse", "thornwall_saloon"} {
		LinkLocationToCity(p, "thornwall", "gate", id)
	}
}

func districtForMillhaven(locID string) string {
	switch locID {
	case "river_docks", "sunlit_lighthouse":
		return "harbor"
	case "millhaven_market", "the_gilded_anchor", "millhaven_town_hall":
		return "market"
	case "temple_of_dawn":
		return "temple_hill"
	case "millhaven_barracks":
		return "garrison"
	case "cutpurse_alley":
		return "undercroft"
	}
	return ""
}

func districtForIronhold(locID string) string {
	if locID == "ironhold_smithy" {
		return "forge_quarter"
	}
	return "keep"
}

func addLoc(p *worldpack.Pack, loc worldpack.Location) {
	AddLocation(p, loc)
}

func buildFactions(p *worldpack.Pack) {
	AddFaction(p, "merchants_league", "Merchants' League",
		"Cartel of trade guilds controlling river tariffs and warehouse licenses.",
		"Maximize profit, monopolize grain routes, keep the vale politically fragmented.")
	AddFaction(p, "order_of_dawn", "Order of the Dawn",
		"Militant faith dedicated to holding back undeath and abyssal corruption from the Shattering.",
		"Seal Undercrypt breaches, support temples, recruit paladins and clerics.")
	AddFaction(p, "red_hand", "Red Hand Bandits",
		"Loose confederation of brigands marked by crimson hand graffiti.",
		"Extort caravans, raid weak settlements, eventually control Thornwall pass.")
}

func buildLore(p *worldpack.Pack) {
	AddLore(p, "lore_shattering", "The Shattering",
		"Five years ago a failed archmage ritual cracked the vale's ley lines. Cities survived; the spaces between them turned hostile. Undead rise faster near fault lines.",
		"", "history", "cataclysm")
	AddLore(p, "lore_silverrun", "The Silverrun River",
		"Major trade artery from Ironhold ore barges to Millhaven's sea gates. River pirates are rare but smugglers common.",
		"sunlit_coast", "trade", "geography")
	AddLore(p, "lore_undercrypt", "Secrets of the Undercrypt",
		"The necropolis predates the empire. Cult of the Hollow Crown seeks a lich regent in the deepest vaults.",
		"undercrypt", "undead", "plot")
	AddLore(p, "lore_whisperwood", "Whispers in the Wood",
		"Shepherds report trees repeating their secrets. Druids say the forest listens to the Shattering and learns fear.",
		"whisperwood", "fey", "horror")
	AddLore(p, "lore_red_hand", "Origins of the Red Hand",
		"Bandit lord Cassian united outcast soldiers after the Shattering. The Hand taxes the moor road and fences stolen League goods in Millhaven's Undercroft.",
		"northern_marches", "bandits", "politics")
}

func buildBestiary(p *worldpack.Pack) {
	srdCreatures := []struct {
		id, name, notes, lore string
		habitats, tags          []string
	}{
		{"creature_goblin", "goblin", "Ambush in packs of 2d4; use Nimble Escape to disengage into underbrush.", "Whisperwood goblins trade with fences in Millhaven.", []string{"forest", "underground", "urban"}, []string{"cr:1/4", "humanoid"}},
		{"creature_kobold", "kobold", "Trap-makers; 1d6 with sling focus fire on spellcasters.", "Kobolds nest in Ironspine mine scrap tunnels.", []string{"mountain", "underground"}, []string{"cr:1/8", "dragonkin"}},
		{"creature_wolf", "wolf", "Pairs patrol Whisperwood trails; pack of 4 near Moonlit Grove.", "Wolves avoid Ironhold ballistae range.", []string{"forest", "grassland"}, []string{"cr:1/4", "beast"}},
		{"creature_giant_rat", "giant rat", "Swarms in shipwreck cove and Undercrypt side tunnels.", "Carries filth fever on crit in home games if desired.", []string{"urban", "coast", "underground"}, []string{"cr:1/8", "beast"}},
		{"creature_bandit", "bandit", "Red Hand standard patrol; demand toll or fight.", "Often led by a thug (use bandit stats with +5 HP).", []string{"road", "grassland", "urban"}, []string{"cr:1/8", "humanoid"}},
		{"creature_guard", "guard", "Town guard stats for Millhaven and Ironhold soldiers.", "Will call reinforcements if outnumbered.", []string{"urban", "fortress"}, []string{"cr:1/8", "humanoid"}},
		{"creature_commoner", "commoner", "Citizens, farmers, dockhands — social encounters.", "Not combatants unless desperate.", []string{"urban", "grassland", "coast"}, []string{"cr:0", "humanoid"}},
		{"creature_skeleton", "skeleton", "Caer Mor ruins at night; 1d6 rise from rubble.", "Vulnerable to bludgeoning — hint via Religion DC 12.", []string{"ruins", "underground", "dungeon"}, []string{"cr:1/4", "undead"}},
		{"creature_zombie", "zombie", "Shambling near Undercrypt entrance; Undead Fortitude surprise.", "Often precedes ghoul packs.", []string{"underground", "ruins", "dungeon"}, []string{"cr:1/4", "undead"}},
		{"creature_ghoul", "ghoul", "Pack of 2d4 in Chamber of Bones; paralyze opens PCs to focus fire.", "Avoid elves — immunity to paralyze.", []string{"underground", "dungeon", "ruins"}, []string{"cr:1", "undead"}},
		{"creature_orc", "orc", "Ironspine raiding parties; Aggressive closes distance fast.", "Sometimes hire out as mercenaries in Thornwall.", []string{"mountain", "grassland"}, []string{"cr:1/2", "humanoid"}},
		{"creature_hobgoblin", "hobgoblin", "Disciplined squads of 4 with longbow volley then melee.", "Martial Advantage punishes clustered PCs.", []string{"mountain", "fortress"}, []string{"cr:1/2", "humanoid"}},
		{"creature_bugbear", "bugbear", "Whisperwood ambush elite; Surprise Attack alpha strike.", "Often paired with goblin scouts.", []string{"forest", "underground"}, []string{"cr:1", "humanoid"}},
		{"creature_ogre", "ogre", "Blocks Ironspine pass alone; dumb but brutal.", "May be bribed with food (Persuasion DC 15).", []string{"mountain", "road"}, []string{"cr:2", "giant"}},
		{"creature_giant_spider", "giant spider", "Webs across Undercrypt antechambers; reuse Stealth +7.", "Web Sense makes bypassing webs tricky.", []string{"forest", "underground", "dungeon"}, []string{"cr:1", "beast"}},
	}
	for _, c := range srdCreatures {
		AddCreatureFromSRD(p, c.id, c.name, c.habitats, c.notes, c.lore, c.tags...)
	}
	// Verify all 17 SRD names present — add any missing from srd.Names()
	seen := map[string]bool{}
	for _, cr := range p.Creatures {
		seen[cr.SRDName] = true
	}
	for _, name := range srd.Names() {
		if !seen[name] {
			id := "creature_" + strings.ReplaceAll(name, " ", "_")
			AddCreatureFromSRD(p, id, name,
				[]string{"wilderness"}, "Generic wilderness encounter.", "Standard SRD creature.", "srd")
		}
	}
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
		AddItem(p, it)
	}
}

func buildNPCs(p *worldpack.Pack) {
	guardSB := mustSRD("guard")
	banditSB := mustSRD("bandit")

	npcs := []worldpack.WorldNPC{
		{
			ID: "npc_eldric_vane", Name: "Mayor Eldric Vane", Role: "quest giver",
			Appearance:  "Silver-haired human in League regalia; signet ring taps the table when thinking.",
			Personality: "Measured, pragmatic, allergic to chaos.",
			Motivations: "Keep Millhaven prosperous and independent from Ironhold.",
			Secrets:     "Secretly funds Dawn expeditions into Undercrypt to appease the temple bloc.",
			Voice:       "Dry baritone, legal metaphors.",
			Knowledge:   []string{"League tariff schedules", "Council votes", "Smuggler routes (partial)"},
			SampleDialogue: []string{
				"The League sells stability by the bushel — don't confuse price with virtue.",
				"If you bring me Cassian's signet, I'll waive your dock fees for a tenday.",
			},
			Disposition: "neutral", FactionID: "merchants_league", DefaultLocation: "millhaven_town_hall",
			Tags: []string{"civic", "noble"},
			ToolBindings: []worldpack.NPCToolBinding{{ToolID: "get_npc", Parameters: map[string]any{"npc_id": "npc_eldric_vane"}}},
		},
		{
			ID: "npc_mira_thorne", Name: "Captain Mira Thorne", Role: "authority",
			Appearance: "Half-elf with scarred cheek; town guard tabard always crisp.",
			Personality: "Direct, fair, zero tolerance for Red Hand graffiti.",
			Motivations: "Protect Millhaven without becoming the League's private army.",
			Secrets:     "Has a informant in Cutpurse Alley (Sable Quinn).",
			Voice:       "Clipped commands; softens slightly for recruits.",
			StatBlock:   &guardSB,
			Disposition: "lawful", FactionID: "order_of_dawn", DefaultLocation: "millhaven_barracks",
			Tags: []string{"military", "guard"},
		},
		{
			ID: "npc_tomas_gull", Name: "Tomas Gull", Role: "informant",
			Appearance: "Broad-shouldered human; apron and anchor tattoo.",
			Personality: "Jovial surface, sharp ears.",
			Motivations: "Keep the Gilded Anchor neutral ground.",
			Knowledge:   []string{"Dock schedules", "Which captains take bribes", "Undercroft entrances"},
			SampleDialogue: []string{"Ale first, questions second — house rule."},
			Disposition: "friendly", DefaultLocation: "the_gilded_anchor",
			Tags: []string{"tavern", "commoner"},
		},
		{
			ID: "npc_lyra_dawn", Name: "Priestess Lyra Dawn", Role: "healer",
			Appearance: "Human woman in white and gold; dawn-symbol tattoo on wrist.",
			Personality: "Compassionate but steel when undead are involved.",
			Motivations: "Close Undercrypt breaches before a death knight rises.",
			StatBlock:   priestessStatBlock(),
			Disposition: "good", FactionID: "order_of_dawn", DefaultLocation: "temple_of_dawn",
			Tags: []string{"cleric", "holy"},
		},
		{
			ID: "npc_brick_holt", Name: "Sergeant Brick Holt", Role: "trainer",
			Appearance: "Stocky dwarf with braided beard and dented helm.",
			Personality: "Gruff mentor energy.",
			Motivations: "Turn green guards into soldiers.",
			StatBlock:   &guardSB,
			DefaultLocation: "millhaven_barracks",
			Tags: []string{"military"},
		},
		{
			ID: "npc_fenn_reed", Name: "Dockmaster Fenn Reed", Role: "merchant",
			Appearance: "Sun-leathered human with rope-calloused hands.",
			Personality: "Everything has a price, including silence.",
			Motivations: "Maximize dock fees; minimize League audits.",
			DefaultLocation: "river_docks", FactionID: "merchants_league",
			Tags: []string{"trade", "dock"},
		},
		{
			ID: "npc_sable_quinn", Name: "Sable Quinn", Role: "fence",
			Appearance: "Hooded half-elf with ink-stained fingers.",
			Personality: "Wry, never surprised.",
			Motivations: "Profit without open war with Captain Thorne.",
			Secrets:     "Red Hand lieutenant but plays both sides.",
			StatBlock:   &banditSB,
			Disposition: "neutral evil", FactionID: "red_hand", DefaultLocation: "cutpurse_alley",
			Tags: []string{"criminal", "rogue"},
		},
		{
			ID: "npc_gareth_ironhold", Name: "Warden Gareth", Role: "authority",
			Appearance: "Human veteran with iron-gray hair and siege-burn scars.",
			Personality: "Soldier first, politician never.",
			Motivations: "Hold the pass; feed the empire's forges.",
			StatBlock:   wardenStatBlock(),
			DefaultLocation: "ironhold_keep",
			Tags: []string{"military", "noble"},
		},
		{
			ID: "npc_helga_stone", Name: "Helga Stone", Role: "merchant",
			Appearance: "Muscular dwarf woman; burn scars on forearms.",
			Personality: "Proud craftswoman; hates shoddy steel.",
			Motivations: "Forge a blade worthy of legend.",
			DefaultLocation: "ironhold_smithy",
			Tags: []string{"craft", "commoner"},
		},
		{
			ID: "npc_jessa_marrow", Name: "Scout-Captain Jessa Marrow", Role: "guide",
			Appearance: "Lean human in mottled cloak; shortbow always strung.",
			Personality: "Dry humor; trusts maps more than people.",
			Motivations: "Keep Thornwall alive through the next winter.",
			StatBlock:   scoutStatBlock(),
			DefaultLocation: "thornwall_gatehouse",
			Tags: []string{"ranger", "frontier"},
		},
		{
			ID: "npc_alden_cross", Name: "Merchant Prince Alden Cross", Role: "merchant",
			Appearance: "Opulent robes; rings on every finger.",
			Personality: "Charming predator.",
			Motivations: "Corner the grain market.",
			FactionID: "merchants_league", DefaultLocation: "millhaven_market",
			Tags: []string{"trade", "noble"},
		},
		{
			ID: "npc_cassian_red", Name: "Cassian of the Red Hand", Role: "villain",
			Appearance: "Crimson cloak; hand brand visible on neck.",
			Personality: "Charismatic bandit king.",
			Motivations: "Control the moor road and ransom Thornwall.",
			StatBlock:   banditLordStatBlock(),
			Secrets:     "Hides at Caer Mor ruins between raids.",
			FactionID: "red_hand", DefaultLocation: "northern_marches_ruins",
			Tags: []string{"villain", "bandit"},
		},
		{
			ID: "npc_nim_willow", Name: "Nim Willow", Role: "healer",
			Appearance: "Gnome with moss-green hair and satchel of herbs.",
			Personality: "Talks to plants; ignores social rank.",
			Motivations: "Protect Whisperwood from logging expeditions.",
			DefaultLocation: "whisperwood_grove",
			Tags: []string{"druid", "herbalist"},
		},
		{
			ID: "npc_durn_kettle", Name: "Durn Kettle", Role: "enforcer",
			Appearance: "Orc with League badge he didn't earn.",
			Personality: "Bullying coward when alone.",
			StatBlock:   mustSRDPtr("orc"),
			FactionID: "merchants_league", DefaultLocation: "millhaven_market",
			Tags: []string{"thug"},
		},
		{
			ID: "npc_mortis", Name: "Brother Mortis", Role: "cultist",
			Appearance: "Hollow-eyed acolyte in crown sigil robes.",
			Personality: "Whispers; believes undeath is mercy.",
			Motivations: "Awaken the Hollow Crown regent.",
			DefaultLocation: "undercrypt_entrance",
			Tags: []string{"cult", "villain"},
		},
		{
			ID: "npc_old_pel", Name: "Old Pel", Role: "hermit",
			Appearance: "Bent human with cataract eyes; smells of fish oil.",
			Personality: "Rambling storyteller.",
			Knowledge:   []string{"Tide charts", "Smuggler signals", "Sea cave layout"},
			DefaultLocation: "sunlit_lighthouse",
			Tags: []string{"commoner", "guide"},
		},
	}
	for _, n := range npcs {
		AddNPC(p, n)
	}
}

func mustSRD(name string) domain.StatBlock {
	sb, ok := srd.Lookup(name)
	if !ok {
		return domain.StatBlock{}
	}
	return sb
}

func mustSRDPtr(name string) *domain.StatBlock {
	sb := mustSRD(name)
	return &sb
}

func priestessStatBlock() *domain.StatBlock {
	return &domain.StatBlock{
		AC: 15, MaxHP: 27, Speed: "30 ft.",
		CR: "2", ProfBonus: 2,
		Skills: []string{"Medicine +4", "Religion +4"},
	}
}

func wardenStatBlock() *domain.StatBlock {
	sb := mustSRD("guard")
	sb.MaxHP = 58
	sb.CR = "3"
	return &sb
}

func scoutStatBlock() *domain.StatBlock {
	sb := mustSRD("bandit")
	sb.Skills = append(sb.Skills, "Survival +4", "Stealth +4", "Perception +3")
	sb.CR = "1/2"
	return &sb
}

func banditLordStatBlock() *domain.StatBlock {
	sb := mustSRD("bandit")
	sb.MaxHP = 45
	sb.CR = "2"
	return &sb
}

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
			LocationID: "cutpurse_alley",
			NPCIDs:     []string{"npc_sable_quinn"},
			ItemIDs:    []string{"item_thieves_tools", "item_red_hand_mask"},
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
			LocationID: "undercrypt_entrance",
			NPCIDs:     []string{"npc_mortis"},
			ItemIDs:    []string{"item_undercrypt_key"},
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
			LocationID: "marches_old_bridge",
			EncounterTableID: "encounter_road_day",
		},
	}
	for _, lc := range contents {
		AddLocationContents(p, lc)
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
		AddEncounterTable(p, t)
	}
}

func buildToolExamples(p *worldpack.Pack) {
	BindToolFromCanonical(p, "get_city", "Millhaven Overview", "Fetch Millhaven city record.", "geography", nil,
		[]worldpack.ToolExample{{Title: "Party arrives", Input: map[string]any{"city_id": "millhaven"}, Output: "Returns districts, population, and location IDs."}})
	BindToolFromCanonical(p, "list_city_locations", "List Millhaven Harbor", "Locations in harbor district.", "geography", nil,
		[]worldpack.ToolExample{{Title: "Find the docks", Input: map[string]any{"city_id": "millhaven", "district_id": "harbor"}, Output: "river_docks, sunlit_lighthouse"}})
	BindToolFromCanonical(p, "get_npc", "Meet Captain Thorne", "Load guard captain NPC.", "population", nil,
		[]worldpack.ToolExample{{Title: "Report crime", Input: map[string]any{"npc_id": "npc_mira_thorne"}, Output: "Personality, stat block, barracks location."}})
	BindToolFromCanonical(p, "roll_encounter_table", "Forest travel roll", "Roll Whisperwood table.", "encounters", nil,
		[]worldpack.ToolExample{{Title: "d12 = 5", Input: map[string]any{"table_id": "encounter_whisperwood", "roll": 5}, Output: "2d4 goblins ambush."}})
	BindToolFromCanonical(p, "search_world", "Search for bandits", "Find Red Hand references.", "reference", nil,
		[]worldpack.ToolExample{{Title: "Query red hand", Input: map[string]any{"query": "Red Hand", "limit": 5}, Output: "npc_cassian_red, lore_red_hand, cutpurse_alley..."}})
}

func buildOracleScenarios(p *worldpack.Pack) {
	p.OracleGuide.Scenarios = []worldpack.GuideScenario{
		{
			Situation: "Party enters Millhaven for the first time",
			UseTools:  []string{"get_city", "list_city_locations", "get_location", "get_lore"},
			Avoid:     []string{"Inventing a new city quarter"},
			InventWhen: "Players ask for a shop type not listed — then invent name but reuse market tags.",
		},
		{
			Situation: "Random forest encounter on the King's Road",
			UseTools:  []string{"roll_encounter_table", "get_creature", "filter_creatures_by_habitat"},
			Avoid:     []string{"Making up new monster stats"},
		},
		{
			Situation: "Player asks who is in the tavern",
			UseTools:  []string{"get_location", "list_location_npcs", "get_npc"},
			Avoid:     []string{"Generating a random tavernkeeper when Gilded Anchor is established"},
		},
		{
			Situation: "Party explores Undercrypt level 1",
			UseTools:  []string{"get_location", "list_location_creatures", "roll_encounter_table", "get_creature"},
			Avoid:     []string{"Skipping undead stat blocks"},
		},
		{
			Situation: "Player wants to buy healing potions",
			UseTools:  []string{"list_location_items", "get_item", "get_location"},
			InventWhen: "Only if party is far from authored shops.",
		},
		{
			Situation: "Travel from Millhaven to Thornwall",
			UseTools:  []string{"describe_travel", "get_region", "roll_encounter_table"},
			Avoid:     []string{"Teleporting encounters without road table"},
		},
		{
			Situation: "Party searches for Red Hand hideout",
			UseTools:  []string{"search_world", "get_faction", "get_lore", "find_nearby_locations"},
			Avoid:     []string{"Relocating Cassian without story reason"},
		},
	}
}
