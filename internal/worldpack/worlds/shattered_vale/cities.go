package shattered_vale

import (
	"github.com/theburrowhub/thaimaturgy/internal/worldpack"
)

func buildCitiesAndLocations(p *worldpack.Pack) {
	// --- Millhaven ---
	millhavenDistricts := []worldpack.District{
		worldpack.AddDistrict("harbor", "Harbor District", "Salt-stained docks and warehouses.", nil, "trade", "docks"),
		worldpack.AddDistrict("market", "Market District", "Bustling bazaar and guild halls.", nil, "trade", "commerce"),
		worldpack.AddDistrict("temple_hill", "Temple Hill", "White stone temples overlooking the river.", nil, "holy"),
		worldpack.AddDistrict("garrison", "Garrison Quarter", "Barracks and the town wall's eastern gate.", nil, "military"),
		worldpack.AddDistrict("undercroft", "Undercroft", "Narrow alleys beneath Temple Hill; thieves and fences.", nil, "criminal"),
	}
	worldpack.AddCity(p, "millhaven", "Millhaven", "sunlit_coast",
		"River-port trade hub of twenty thousand souls. The Merchants' League holds sway, but the Order of the Dawn keeps the peace.",
		millhavenDistricts, "trade", "port", "hub")
	p.Cities[0].Population = "~20,000"
	p.Cities[0].Government = "Merchant Council chaired by Mayor Eldric Vane"
	p.Cities[0].MapID = "map_millhaven"
	worldpack.LinkCityToRegion(p, "sunlit_coast", "millhaven")

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
	worldpack.AddCity(p, "ironhold", "Ironhold", "ironspine_mountains",
		"Mountain fortress-city controlling ore routes. Warden Gareth rules with military pragmatism.",
		[]worldpack.District{
			worldpack.AddDistrict("keep", "High Keep", "Citadel and officer quarters.", nil, "military"),
			worldpack.AddDistrict("forge_quarter", "Forge Quarter", "Smelters and master smiths.", nil, "craft"),
		}, "fortress", "mining")
	p.Cities[1].Population = "~8,000"
	p.Cities[1].Government = "Military governorship under Warden Gareth"
	p.Cities[1].MapID = "map_ironhold"
	worldpack.LinkCityToRegion(p, "ironspine_mountains", "ironhold")

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
	worldpack.AddCity(p, "thornwall", "Thornwall", "northern_marches",
		"Palisade frontier town guarding the moor road. Last respectable stop before the wild marches.",
		[]worldpack.District{
			worldpack.AddDistrict("gate", "Gate Town", "Inns and traders serving caravan traffic.", nil, "frontier"),
		}, "frontier", "caravan")
	p.Cities[2].Population = "~2,500"
	p.Cities[2].Government = "Frontier charter; Scout-Captain Jessa Marrow holds practical authority"
	worldpack.LinkCityToRegion(p, "northern_marches", "thornwall")

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
			RegionID:    "whisperwood",
			Description: "Circle of silver-barked trees; fey lights at new moon.",
			ReadAloud:   "The forest hushes. Pale mushrooms glow between roots like scattered coins.",
			Tags:        []string{"forest", "fey", "wilderness"},
			Connections: []string{"whisperwood_roadside_shrine", "marches_old_bridge"},
		},
		{
			ID: "whisperwood_roadside_shrine", Name: "King's Road Shrine", Kind: "shrine",
			RegionID:    "whisperwood",
			Description: "Weathered saint statue; travelers leave coins for safe passage.",
			ReadAloud:   "A cracked saint watches the road. Offerings rust in a stone bowl.",
			Tags:        []string{"forest", "road", "holy"},
			Connections: []string{"whisperwood_grove", "marches_old_bridge"},
		},
		{
			ID: "ironspine_pass", Name: "Ironspine High Pass", Kind: "wilderness",
			RegionID:    "ironspine_mountains",
			Description: "Wind-scoured pass; hobgoblin patrols at dusk.",
			ReadAloud:   "The path narrows along a cliff. Far below, a river cuts through fog.",
			Tags:        []string{"mountain", "travel", "danger"},
			Connections: []string{"ironhold_training_yard"},
		},
		{
			ID: "sunlit_coast_cliffs", Name: "Sunlit Cliffs", Kind: "wilderness",
			RegionID:    "sunlit_coast",
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
			RegionID:    "northern_marches",
			Description: "Collapsed imperial fort; skeletons and shadows at night.",
			ReadAloud:   "Broken towers claw at low clouds. Ivy masks arrow slits like closed eyes.",
			Tags:        []string{"ruins", "undead", "dungeon"},
			Connections: []string{"marches_old_bridge", "undercrypt_entrance"},
		},
		{
			ID: "marches_old_bridge", Name: "Old Moor Bridge", Kind: "wilderness",
			RegionID:    "northern_marches",
			Description: "Stone bridge over a black river; toll collectors replaced by bandits some nights.",
			ReadAloud:   "Ancient arches span sluggish water. Carved wards have been chiseled away.",
			Tags:        []string{"road", "river", "bandits"},
			Connections: []string{"thornwall_gatehouse", "whisperwood_grove", "northern_marches_ruins"},
		},
		{
			ID: "coast_shipwreck_cove", Name: "Shipwreck Cove", Kind: "wilderness",
			RegionID:    "sunlit_coast",
			Description: "Hidden cove with rotting hulls; giant rats and smuggler caches.",
			ReadAloud:   "Barnacle-crusted ribs of ships lean over black sand. The tide sucks at broken timbers.",
			Tags:        []string{"coast", "smuggling", "dungeon"},
			Connections: []string{"sunlit_coast_cliffs"},
		},
		{
			ID: "undercrypt_entrance", Name: "Collapsed Temple Entrance", Kind: "dungeon",
			RegionID:    "undercrypt",
			Description: "Sinkhole revealing stairways into the necropolis. Cult graffiti marks the walls.",
			ReadAloud:   "Cold air exhales from broken flagstones. Torchlight does not reach the bottom.",
			Tags:        []string{"dungeon", "undead", "underground"},
			Connections: []string{"northern_marches_ruins", "undercrypt_chamber_of_bones"},
		},
		{
			ID: "undercrypt_chamber_of_bones", Name: "Chamber of Bones", Kind: "dungeon",
			RegionID:    "undercrypt",
			Description: "Ossuary chamber; ghoul packs nest behind rib-cage arches.",
			ReadAloud:   "Skulls piled to the ceiling watch with empty sockets. Scraping echoes from side tunnels.",
			Tags:        []string{"dungeon", "undead", "underground"},
			Connections: []string{"undercrypt_entrance"},
		},
	}
	for _, loc := range wildLocs {
		addLoc(p, loc)
		worldpack.LinkWildernessLocation(p, loc.RegionID, loc.ID)
	}

	// Link city locations
	for _, id := range []string{
		"millhaven_market", "the_gilded_anchor", "temple_of_dawn", "millhaven_barracks",
		"river_docks", "cutpurse_alley", "millhaven_town_hall",
	} {
		worldpack.LinkLocationToCity(p, "millhaven", districtForMillhaven(id), id)
	}
	for _, id := range []string{"ironhold_keep", "ironhold_smithy", "ironhold_training_yard"} {
		worldpack.LinkLocationToCity(p, "ironhold", districtForIronhold(id), id)
	}
	for _, id := range []string{"thornwall_gatehouse", "thornwall_saloon"} {
		worldpack.LinkLocationToCity(p, "thornwall", "gate", id)
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
	worldpack.AddLocation(p, loc)
}
