package caribdus

import (
	"encoding/json"

	"github.com/theburrowhub/thaimaturgy/internal/domain"
	"github.com/theburrowhub/thaimaturgy/internal/worldpack"
)

// Build returns the Caribdus flooded-archipelago world pack.
func Build() *worldpack.Pack {
	p := worldpack.NewBaseWorld("caribdus", "Caribdus", worldpack.WorldMeta{
		SettingName:         "Caribdus",
		Summary:             "A drowned archipelago where colonial fleets, pirate republics, and sea-witch covens fight over reef-choked waters and cursed treasure.",
		SuggestedRulesystem: "savage_worlds",
		PlayableWith:        []string{"savage_worlds", "dnd5e", "d100"},
	})

	worldpack.SetSettingTone(p,
		"Sailing age — flintlocks, powder, and rope; no heavy industry",
		"Swashbuckling dread; salt spray, superstition, and sudden violence",
		"Caribdus is a chain of storm-battered isles half-swallowed by the Deluge. Every cove hides a flag, every reef hides teeth.",
		"nautical", "pirates", "sea-witch", "colonial",
	)

	worldpack.SetWorldRulesFull(p, worldpack.WorldRules{
		Magic:      "Common — sea-witch hedge magic, cursed relics from the Deluge, and tide-bound rituals. Arcane academies are absent; power smells of brine and blood.",
		Technology: "Sailing age — flintlock pistols, cutlasses, schooners, and diving bells. No steam, no mass production; shipyards hand-fit every hull.",
		Travel:     "Open sea between isles; reef pilots required in the archipelago; Ghost Shoals warp compasses.",
	})

	worldpack.SetPoliticsFull(p, worldpack.Politics{
		Summary: "Colonial armadas claim harbors with cannon and charter; pirate republics vote captains by steel and spoils; sea-witch covens trade storms for souls along the Ghost Shoals.",
		MajorPowers: []string{
			"Armada del Rey (Fuerte Almirez)",
			"República de Perla Azul",
			"Capitanes Libres (Puerto Sombrío)",
			"Coven del Mar Espectral",
		},
		Conflicts: []string{
			"Tariffs vs smuggling routes",
			"Pirate raids vs colonial patrols",
			"Sea-witch bargains vs temple inquisitors",
		},
	})

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
	addRegion(p, "open_sea", "Open Sea",
		"Trackless swells between island chains. Squalls rise without warning; hulls groan on long swells.",
		"ocean", "travel", "naval")
	p.Regions[0].TravelNotes = "Navigation checks each day; off-course drift toward Ghost Shoals on failed rolls."
	p.Regions[0].MapID = "map_open_sea"

	addRegion(p, "central_archipelago", "Central Archipelago",
		"The heart of Caribdus — crowded anchorages, reef mazes, and the three great ports.",
		"archipelago", "trade", "civilized")
	p.Regions[1].CityIDs = []string{"puerto_sombrio", "perla_azul", "fuerte_almirez"}
	p.Regions[1].TravelNotes = "Inter-island hops take hours by sloop; reef pilots charge 5 gp per passage."
	p.Regions[1].MapID = "map_central_archipelago"

	addRegion(p, "ghost_shoals", "Ghost Shoals",
		"Mist-wrapped shallows where compasses spin and drowned bells toll at dusk.",
		"mist", "undead", "supernatural")
	p.Regions[2].TravelNotes = "Visibility rarely exceeds 100 ft.; sea-witch pacts rumored to calm the fog."
	p.Regions[2].MapID = "map_ghost_shoals"

	addRegion(p, "deep_trench", "Deep Trench",
		"A black scar in the ocean floor. Bioluminescent hunters and Deluge ruins lurk below the thermocline.",
		"deep_ocean", "horror", "ruins")
	p.Regions[3].TravelNotes = "Surface ships avoid the trench; diving bells and enchanted lungs are mandatory."
	p.Regions[3].MapID = "map_deep_trench"
}

func buildMaps(p *worldpack.Pack) {
	addMap(p, "map_central_archipelago", "Central Archipelago Chart", "regional",
		"maps/central_archipelago.png", "Reef channels, trade winds, and the three port cities marked with anchor symbols.", "1 hex = 8 nautical miles")
	addMap(p, "map_puerto_sombrio", "Puerto Sombrío Harbor", "city",
		"maps/puerto_sombrio.png", "Colonial walls, black-market docks, and the shipyard cranes.", "1 square = 50 ft")
	addMap(p, "map_perla_azul", "Perla Azul Lagoon", "city",
		"maps/perla_azul.png", "Pearl market, tide temple, and coral breakwater.", "1 square = 50 ft")
	addMap(p, "map_fuerte_almirez", "Fuerte Almirez Citadel", "city",
		"maps/fuerte_almirez.png", "Naval fort, governor's plaza, and ironclad shipyard.", "1 square = 50 ft")
	addMap(p, "map_ghost_shoals", "Ghost Shoals Mist Chart", "regional",
		"maps/ghost_shoals.png", "Shallow wrecks, witch huts, and tide rips hatched in gray.", "1 hex = 4 nautical miles")
}

func buildCitiesAndLocations(p *worldpack.Pack) {
	// --- Puerto Sombrío ---
	sombrioDistricts := []worldpack.District{
		addDistrict("muelle_viejo", "Muelle Viejo", "Rotting piers and tariff shacks; first sight of Caribdus for many crews.", nil, "docks", "trade"),
		addDistrict("barrio_velas", "Barrio de las Velas", "Sail-loft alleys, chandlers, and boarding houses.", nil, "maritime", "urban"),
		addDistrict("muralla_colonial", "Muralla Colonial", "Stone walls, customs house, and the governor's shadow.", nil, "colonial", "law"),
	}
	addCity(p, "puerto_sombrio", "Puerto Sombrío", "central_archipelago",
		"Grim port of fifteen thousand souls where Crown tariffs meet pirate gold. Smoke and tar hang over every dawn.",
		sombrioDistricts, "port", "colonial", "smuggling")
	p.Cities[0].Population = "~15,000"
	p.Cities[0].Government = "Colonial governor with a bribed council"
	p.Cities[0].MapID = "map_puerto_sombrio"
	linkCityToRegion(p, "central_archipelago", "puerto_sombrio")

	addLoc(p, worldpack.Location{
		ID: "taberna_ancla_podrida", Name: "Taberna del Ancla Podrida", Kind: "tavern",
		CityID: "puerto_sombrio", DistrictID: "muelle_viejo", RegionID: "central_archipelago",
		Description: "Three-story flophouse favored by bosuns and deserters. Rooms 4 sp; grog 2 cp; stew questionable.",
		ReadAloud:   "A barnacle-crusted anchor hangs above the door. Inside, dice clatter and a concertina wheezes through pipe smoke.",
		DMNotes:     "Tabernero Paco hears every dock rumor. Hidden hatch in the cellar leads toward smuggler tunnels.",
		Tags:        []string{"tavern", "rest", "rumors"},
		Connections: []string{"astillero_negro", "mercado_especias"},
	})
	addLoc(p, worldpack.Location{
		ID: "astillero_negro", Name: "Astillero Negro", Kind: "shipyard",
		CityID: "puerto_sombrio", DistrictID: "muelle_viejo", RegionID: "central_archipelago",
		Description: "Black-tarred slips where hulls are patched between raids. Contramaestre Toro runs the night shift.",
		ReadAloud:   "Hammer on copper rivets rings across oily water. Cranes swing masts like matchsticks.",
		DMNotes:     "Stolen naval timber stored under tarpaulin 7. Athletics DC 14 to slip aboard a refitting sloop.",
		Tags:        []string{"shipyard", "craft", "travel"},
		Connections: []string{"taberna_ancla_podrida", "cala_contrabandistas"},
	})
	addLoc(p, worldpack.Location{
		ID: "mercado_especias", Name: "Mercado de Especias", Kind: "market",
		CityID: "puerto_sombrio", DistrictID: "barrio_velas", RegionID: "central_archipelago",
		Description: "Canvas stalls selling pepper, rum, compass needles, and questionable charms.",
		ReadAloud:   "Spice heat and tar compete in the air. Hawkers bark prices in three languages.",
		Tags:        []string{"market", "shopping", "social"},
		Connections: []string{"taberna_ancla_podrida", "aduana_sombrio"},
	})
	addLoc(p, worldpack.Location{
		ID: "aduana_sombrio", Name: "Aduana Colonial", Kind: "civic",
		CityID: "puerto_sombrio", DistrictID: "muralla_colonial", RegionID: "central_archipelago",
		Description: "Customs house where Crown clerks stamp manifests and squeeze bribes.",
		ReadAloud:   "Wax seals and iron grilles. A clerk's quill scratches like a rat in the wall.",
		DMNotes:     "Governor's envoy visits on high tide. Hidden ledger of smuggler payoffs in the vault.",
		Tags:        []string{"civic", "law", "colonial"},
		Connections: []string{"mercado_especias", "fortin_colonial"},
	})
	addLoc(p, worldpack.Location{
		ID: "fortin_colonial", Name: "Fortín Colonial", Kind: "fort",
		CityID: "puerto_sombrio", DistrictID: "muralla_colonial", RegionID: "central_archipelago",
		Description: "Cannon battery guarding the harbor mouth. Powder magazine below the parapet.",
		ReadAloud:   "Salt-crusted cannon stare seaward. Red-coated marines drill in the yard.",
		Tags:        []string{"military", "fort"},
		Connections: []string{"aduana_sombrio"},
	})

	// --- Perla Azul ---
	perlaDistricts := []worldpack.District{
		addDistrict("mercado_perlas", "Mercado de Perlas", "Open-air auction pits and diver guild halls.", nil, "trade", "pearl"),
		addDistrict("malecon_dorado", "Malecón Dorado", "Wealthy villas on the lagoon rim.", nil, "noble", "coast"),
		addDistrict("barrio_pescador", "Barrio Pescador", "Net menders, shrines, and cheap ceviche stalls.", nil, "fishing", "urban"),
	}
	addCity(p, "perla_azul", "Perla Azul", "central_archipelago",
		"Lagoon city built on coral pilings. Pearl trade and tide worship keep the peace — barely.",
		perlaDistricts, "port", "trade", "temple")
	p.Cities[1].Population = "~9,000"
	p.Cities[1].Government = "Pearl Council of master divers and temple elders"
	p.Cities[1].MapID = "map_perla_azul"
	linkCityToRegion(p, "central_archipelago", "perla_azul")

	addLoc(p, worldpack.Location{
		ID: "taberna_perla", Name: "Taberna de la Perla", Kind: "tavern",
		CityID: "perla_azul", DistrictID: "mercado_perlas", RegionID: "central_archipelago",
		Description: "Diver's tavern with a pearl-inlaid bar. Fresh catch served at dawn.",
		ReadAloud:   "Mother-of-pearl gleams beneath tankards. Diver tattoos flash in lamplight.",
		Tags:        []string{"tavern", "rest"},
		Connections: []string{"casa_cambio_perla", "templo_mareas"},
	})
	addLoc(p, worldpack.Location{
		ID: "casa_cambio_perla", Name: "Casa de Cambio del Arrecife", Kind: "shop",
		CityID: "perla_azul", DistrictID: "mercado_perlas", RegionID: "central_archipelago",
		Description: "Money-changer and fence for unmarked pearls. Heavy shutters, heavier guards.",
		ReadAloud:   "Scales ping. A grille slides open just wide enough for hands and coin.",
		Tags:        []string{"shop", "finance"},
		Connections: []string{"taberna_perla"},
	})
	addLoc(p, worldpack.Location{
		ID: "templo_mareas", Name: "Templo de las Mareas", Kind: "temple",
		CityID: "perla_azul", DistrictID: "malecon_dorado", RegionID: "central_archipelago",
		Description: "Open-air shrine where tide priests read omens in shell patterns.",
		ReadAloud:   "Conch horns echo at dusk. Incense of seaweed and myrrh drifts over the lagoon.",
		DMNotes:     "Sacerdotisa Coral can bless water-breathing for a tithe.",
		Tags:        []string{"temple", "holy", "sea-witch"},
		Connections: []string{"taberna_perla", "malecon_faro"},
	})
	addLoc(p, worldpack.Location{
		ID: "malecon_faro", Name: "Faro del Malecón", Kind: "lighthouse",
		CityID: "perla_azul", DistrictID: "malecon_dorado", RegionID: "central_archipelago",
		Description: "White lighthouse guiding lagoon traffic. Cartographer's workshop in the base.",
		ReadAloud:   "A beam sweeps over coral breakers. Charts rustle in a salt-stained room below.",
		Tags:        []string{"navigation", "landmark"},
		Connections: []string{"templo_mareas"},
	})

	// --- Fuerte Almirez ---
	almirezDistricts := []worldpack.District{
		addDistrict("plaza_armas", "Plaza de Armas", "Parade ground and governor's palace.", nil, "civic", "military"),
		addDistrict("astilleros_navales", "Astilleros Navales", "Crown shipyard and rope walks.", nil, "shipyard", "naval"),
		addDistrict("cuartel_naval", "Cuartel Naval", "Barracks, powder store, and naval court.", nil, "military", "law"),
	}
	addCity(p, "fuerte_almirez", "Fuerte Almirez", "central_archipelago",
		"Stone citadel and naval anchor of the Crown Armada. Discipline, cannon, and colonial ambition.",
		almirezDistricts, "fortress", "naval", "colonial")
	p.Cities[2].Population = "~12,000"
	p.Cities[2].Government = "Governor-General Mateo Almirez"
	p.Cities[2].MapID = "map_fuerte_almirez"
	linkCityToRegion(p, "central_archipelago", "fuerte_almirez")

	addLoc(p, worldpack.Location{
		ID: "palacio_gobernador", Name: "Palacio del Gobernador", Kind: "civic",
		CityID: "fuerte_almirez", DistrictID: "plaza_armas", RegionID: "central_archipelago",
		Description: "Marble-faced palace where colonial edicts become law.",
		ReadAloud:   "Guards in blue coats flank a gate of black iron. Fanfare is optional; taxes are not.",
		Tags:        []string{"civic", "noble", "quests"},
		Connections: []string{"cuartel_almirez", "astillero_crown"},
	})
	addLoc(p, worldpack.Location{
		ID: "cuartel_almirez", Name: "Cuartel del Gobernador", Kind: "barracks",
		CityID: "fuerte_almirez", DistrictID: "cuartel_naval", RegionID: "central_archipelago",
		Description: "Naval barracks housing two hundred marines. Armory issues weapons by writ only.",
		ReadAloud:   "Boot heels crack on stone. The smell of gun oil is stronger than the sea.",
		Tags:        []string{"military", "law"},
		Connections: []string{"palacio_gobernador", "cantina_mosquito"},
	})
	addLoc(p, worldpack.Location{
		ID: "astillero_crown", Name: "Astillero de la Corona", Kind: "shipyard",
		CityID: "fuerte_almirez", DistrictID: "astilleros_navales", RegionID: "central_archipelago",
		Description: "Crown shipyard building frigates and powder hulks.",
		ReadAloud:   "Sawdust snows on the quay. A frigate skeleton waits for its copper sheathing.",
		Tags:        []string{"shipyard", "naval"},
		Connections: []string{"palacio_gobernador"},
	})
	addLoc(p, worldpack.Location{
		ID: "cantina_mosquito", Name: "Cantina del Mosquito", Kind: "tavern",
		CityID: "fuerte_almirez", DistrictID: "cuartel_naval", RegionID: "central_archipelago",
		Description: "Off-duty naval tavern. Darts, dice, and discreet duels in the alley.",
		ReadAloud:   "A mosquito is nailed above the door — wings spread, iron pin through the thorax.",
		Tags:        []string{"tavern", "social"},
		Connections: []string{"cuartel_almirez"},
	})

	// --- Wilderness & sea locations ---
	addLoc(p, worldpack.Location{
		ID: "arrecife_coral", Name: "Arrecife de Coral Sangriento", Kind: "wilderness",
		RegionID: "central_archipelago",
		Description: "Crimson coral maze that tears hulls. Sharks patrol the outer rim.",
		ReadAloud:   "Breakers hiss over coral teeth. The lagoon inside is eerily calm.",
		Tags:        []string{"reef", "wilderness", "danger"},
		Connections: []string{"cala_contrabandistas"},
	})
	linkWildernessLocation(p, "central_archipelago", "arrecife_coral")

	addLoc(p, worldpack.Location{
		ID: "cala_contrabandistas", Name: "Cala de los Contrabandistas", Kind: "wilderness",
		RegionID: "central_archipelago",
		Description: "Hidden cove with camouflaged nets and a pulley hoist into the cliffs.",
		ReadAloud:   "Tarps mimic rock from the sea. A whistle answers your approach — friend or foe?",
		DMNotes:     "Garfio Reyes uses this cove. Smuggler lookouts Perception +4.",
		Tags:        []string{"smuggling", "cove", "wilderness"},
		Connections: []string{"astillero_negro", "arrecife_coral"},
	})
	linkWildernessLocation(p, "central_archipelago", "cala_contrabandistas")

	addLoc(p, worldpack.Location{
		ID: "choza_bruja_mar", Name: "Choza de la Bruja del Mar", Kind: "wilderness",
		RegionID: "ghost_shoals",
		Description: "Stilt hut on a dead mangrove. Shell wind chimes and cauldron smoke.",
		ReadAloud:   "A hut squats on stilts above black water. Chimes clack though there is no wind.",
		DMNotes:     "Marisela la Bruma trades storms for favors. Coven audience by invitation only.",
		Tags:        []string{"sea-witch", "wilderness", "magic"},
	})
	linkWildernessLocation(p, "ghost_shoals", "choza_bruja_mar")

	addLoc(p, worldpack.Location{
		ID: "naufragio_espectral", Name: "Naufragio Espectral", Kind: "wilderness",
		RegionID: "ghost_shoals",
		Description: "Ghostly galleon fused with the reef. Bells toll at midnight.",
		ReadAloud:   "Mist parts around a half-sunk hull. Rigging moves with no crew aboard.",
		Tags:        []string{"undead", "wreck", "wilderness"},
	})
	linkWildernessLocation(p, "ghost_shoals", "naufragio_espectral")

	addLoc(p, worldpack.Location{
		ID: "faro_tormenta", Name: "Faro de la Tormenta", Kind: "lighthouse",
		RegionID: "open_sea",
		Description: "Storm-lashed lighthouse on a bare rock. Keeper hasn't been paid in years.",
		ReadAloud:   "Lightning fractures the sky. The beacon still turns — who fuels it?",
		Tags:        []string{"lighthouse", "open_sea", "mystery"},
	})
	linkWildernessLocation(p, "open_sea", "faro_tormenta")

	addLoc(p, worldpack.Location{
		ID: "templo_sumergido", Name: "Templo Sumergido del Abismo", Kind: "dungeon",
		RegionID: "deep_trench",
		Description: "Deluge-era temple half buried in the trench wall. Air pockets hold stale breath.",
		ReadAloud:   "Stone angels stare through green dark. Your lamp throws shadows that move wrong.",
		Tags:        []string{"ruins", "deep", "dungeon"},
	})
	linkWildernessLocation(p, "deep_trench", "templo_sumergido")

	addLoc(p, worldpack.Location{
		ID: "fosa_abisal", Name: "Fosa Abisal de las Luces", Kind: "wilderness",
		RegionID: "deep_trench",
		Description: "Bioluminescent vent field. Pressure and predators end careless dives.",
		ReadAloud:   "Cold light blooms in the black. Something vast shifts below your keel.",
		Tags:        []string{"deep", "horror", "wilderness"},
	})
	linkWildernessLocation(p, "deep_trench", "fosa_abisal")

	// Link all city locations to districts
	for _, loc := range p.Locations {
		if loc.CityID != "" {
			linkLocationToCity(p, loc.CityID, loc.DistrictID, loc.ID)
		}
	}
}

func buildFactions(p *worldpack.Pack) {
	addFaction(p, "crown_armada", "Crown Armada",
		"Colonial navy and marines enforcing tariffs, charters, and hangings.",
		"Control every harbor; crush pirate republics; monopolize powder and ship timber.")
	addFaction(p, "free_corsair_council", "Free Corsair Council",
		"Loose confederation of pirate captains voting at Tortuga-style moots.",
		"Keep ports free of Crown law; split spoils fairly; hang traitors from the yardarm.")
	addFaction(p, "coven_tides", "Coven of Tides",
		"Sea-witch sisterhood binding storms, curses, and tidal omens.",
		"Protect Ghost Shoals; punish oath-breakers; recover Deluge relics from the trench.")
}

func buildLore(p *worldpack.Pack) {
	addLore(p, "lore_great_deluge", "The Great Deluge",
		"Three centuries ago the sea rose in a single season, swallowing empires and leaving Caribdus a reef-strewn graveyard. Sailors say the trench still belches the bones of drowned cities.",
		"central_archipelago", "history", "deluge")
	addLore(p, "lore_broken_anchors", "Treaty of Broken Anchors",
		"A fragile truce between Crown Armada and the Corsair Council: no firing in designated anchorages, exchange of prisoners, and shared charts through neutral Perla Azul. Both sides cheat.",
		"central_archipelago", "politics", "treaty")
	addLore(p, "lore_black_pearl_curse", "Curse of the Black Pearl",
		"A jet pearl taken from the trench is said to weigh down a captain's soul. Ships carrying one never outrun the same storm twice.",
		"deep_trench", "curse", "treasure")
	addLore(p, "lore_shoal_bells", "Bells of the Ghost Shoals",
		"Drowned sailors ring wrecks at dusk. Hearing the bells thrice marks you for the sea hags unless a witch breaks the omen with salt and blood.",
		"ghost_shoals", "supernatural", "undead")
}

func buildBestiary(p *worldpack.Pack) {
	addCreatureSavage(p, "creature_reef_shark", "Reef Shark",
		savageProfile("Normal", "A", "F", "6", "8", "Swim 12", "d6 bite", "Pack hunter; +2 attack when blood in water"),
		[]string{"reef", "coast", "open_sea"}, "1d4+1 in bloodied water near reefs.", "Caribdus sailors call them 'red fins'.", "beast", "aquatic")
	addCreatureSavage(p, "creature_giant_octopus", "Giant Octopus",
		savageProfile("Large", "A", "G", "8", "12", "Swim 8", "2d6 tentacle", "Grapple on raise; ink cloud"),
		[]string{"reef", "deep", "ship"}, "One per sunken wreck or night dive.", "Smugglers train juveniles as living mooring lines — poorly.", "beast", "aquatic")
	addCreatureSavage(p, "creature_drowned_sailor", "Drowned Sailor",
		savageProfile("Normal", "A", "F", "6", "8", "Swim 6", "d6 rust-cutlass", "Undead; Fear check on first sight"),
		[]string{"ghost_shoals", "wreck", "open_sea"}, "2d6 rising from spectral wreck at dusk.", "Crew of the Naufragio Espectral.", "undead", "aquatic")
	addCreatureSavage(p, "creature_bruma_marina", "Bruma Marina",
		savageProfile("Normal", "A", "G", "8", "10", "Swim 8", "d6 claws", "Sea hag analog; ill omen; curse on wound"),
		[]string{"ghost_shoals", "mist"}, "Solitary in shoals; with 1d4 drowned sailors.", "Covens deny kinship; sailors disagree.", "monstrosity", "sea-witch")
	addCreatureSavage(p, "creature_pirate_crew", "Pirate Crew",
		savageProfile("Normal", "A", "F", "6", "6", "Pace 6", "d6 cutlass", "Wild Attack optional; gang up"),
		[]string{"urban", "coast", "open_sea"}, "Crew of 6–12 with one Wild Card captain.", "Mix of Corsair Council and freelancers.", "humanoid", "pirate")
	addCreatureSavage(p, "creature_giant_crab", "Giant Crab",
		savageProfile("Large", "A", "G", "8", "14", "Pace 6", "2d6 pincer", "Hard shell +2 armor"),
		[]string{"reef", "coast"}, "1–2 on tidal flats or pearl beds.", "Perla Azul divers carry hammers specifically for them.", "beast", "aquatic")
	addCreatureSavage(p, "creature_jellyfish_swarm", "Jellyfish Swarm",
		savageProfile("Large", "A", "G", "6", "8", "Swim 4", "d4 sting", "Touch attack; poison fatigue"),
		[]string{"reef", "open_sea", "deep"}, "Swarm in warm currents; Perception DC 12 to spot.", "Called 'crown's lace' by Almirez marines.", "beast", "aquatic")
	addCreatureSavage(p, "creature_ghost_pirate", "Ghost Pirate",
		savageProfile("Normal", "A", "G", "8", "10", "Pace 6", "d8 ethereal cutlass", "Undead; immune normal weapons unless silver or magic"),
		[]string{"ghost_shoals", "wreck"}, "1 Wild Card with 1d6 crew shadows.", "Officers of the Naufragio Espectral.", "undead", "pirate")
	addCreatureSavage(p, "creature_leviathan_spawn", "Leviathan Spawn",
		savageProfile("Huge", "A", "G", "12", "24", "Swim 10", "3d6 bite", "Swallow whole on raise+4"),
		[]string{"deep", "open_sea"}, "Solitary near trench; triggers Fear.", "Admirals deny reports; divers do not.", "monstrosity", "horror")
	addCreatureSavage(p, "creature_barnacle_zombie", "Barnacle Zombie",
		savageProfile("Normal", "A", "F", "6", "10", "Pace 4", "d6 slam", "Undead Fortitude analog; slow"),
		[]string{"wreck", "ghost_shoals", "ship"}, "2d4 aboard abandoned hulks.", "Failsafe guardians for smuggler scuttling.", "undead", "aquatic")
	addCreatureSavage(p, "creature_sea_witch_familiar", "Sea Witch Familiar",
		savageProfile("Small", "A", "G", "6", "6", "Fly 8", "d4 talons", "Spell support; binds to coven hag"),
		[]string{"ghost_shoals", "urban"}, "With Marisela or temple envoys.", "Osprey, heron, or skeletal gull.", "beast", "familiar")
	addCreatureSavage(p, "creature_crown_marine", "Crown Marine",
		savageProfile("Normal", "A", "F", "6", "8", "Pace 6", "d8 flintlock", "Volley fire +1 when in squad of 4+"),
		[]string{"urban", "fort", "naval"}, "Squad of 4–8 with sergeant Wild Card.", "Fuerte Almirez issue.", "humanoid", "military")
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
		addItem(p, it)
	}
}

func buildNPCs(p *worldpack.Pack) {
	npcs := []worldpack.WorldNPC{
		{
			ID: "npc_valeria_storm", Name: "Capitán Valeria Storm", Role: "privateer captain",
			Appearance:  "Lean woman, storm-gray coat, silver earring shaped like a lightning bolt.",
			Personality: "Bold, theatrical, hates Crown hypocrisy.",
			Motivations: "Build a fleet that answers to no flag but her own.",
			Secrets:     "Owes Marisela a blood debt from a saved crew.",
			Voice:       "Laughs mid-sentence; calls everyone 'mate' until they earn a name.",
			Knowledge:   []string{"Reef passages", "Corsair Council votes", "Storm omens"},
			SampleDialogue: []string{
				"I don't sail for kings or coffers — I sail because the sea asked nicely.",
				"Puerto Sombrío's customs man takes bribes in pearls. I take his pride for free.",
			},
			Disposition: "chaotic good", FactionID: "free_corsair_council", DefaultLocation: "taberna_ancla_podrida",
			StatBlock:   ptrStatBlock(captainStatBlock()),
			Tags:        []string{"captain", "wild_card"},
		},
		{
			ID: "npc_marisela_bruma", Name: "Marisela la Bruma", Role: "sea witch",
			Appearance:  "Salt-white hair; kelp bracelets; eyes like tide pools.",
			Personality: "Patient, ominous, never wastes a word.",
			Motivations: "Bind the trench's waking leviathan back to sleep.",
			Secrets:     "Sold calm weather to the Crown once; regrets it.",
			Voice:       "Speaks as if every sentence is a prophecy.",
			Knowledge:   []string{"Ghost Shoals paths", "Curse breaking", "Deluge glyphs"},
			SampleDialogue: []string{
				"The sea remembers every oath you break. I merely read the minutes.",
				"Bring salt, blood, and silence. Questions come after.",
			},
			Disposition: "neutral", FactionID: "coven_tides", DefaultLocation: "choza_bruja_mar",
			StatBlock:   ptrStatBlock(seaWitchStatBlock()),
			Tags:        []string{"sea-witch", "spellcaster"},
		},
		{
			ID: "npc_mateo_almirez", Name: "Gobernador Mateo Almirez", Role: "colonial governor",
			Appearance:  "Iron-gray uniform, gout limp, signet ring of the Crown.",
			Personality: "Polished cruelty; believes order is mercy.",
			Motivations: "Hang enough pirates to make an example; fill the treasury.",
			Secrets:     "Secretly trades prisoners to the coven for storm warnings.",
			Voice:       "Soft Spanish cadence; threats wrapped in etiquette.",
			Disposition: "lawful evil", FactionID: "crown_armada", DefaultLocation: "palacio_gobernador",
			Tags:        []string{"noble", "authority"},
		},
		{
			ID: "npc_garfio_reyes", Name: "Garfio Reyes", Role: "smuggler",
			Appearance:  "Hook-hand prosthetic (iron, not silver); shark-tooth necklace.",
			Personality: "Grins through danger; loyal to coin.",
			Motivations: "Control the Sombrío smuggling routes.",
			Secrets:     "Informant for Valeria Storm when Crown tariffs rise.",
			Voice:       "Whispers like a knife sliding from a sheath.",
			StatBlock:   ptrStatBlock(pirateStatBlock()),
			Disposition: "neutral", FactionID: "free_corsair_council", DefaultLocation: "cala_contrabandistas",
			Tags:        []string{"criminal", "smuggler"},
		},
		{
			ID: "npc_toro_contramaestre", Name: "Contramaestre Toro", Role: "shipwright",
			Appearance:  "Massive shoulders, tar-stained beard, voice like a capstan.",
			Personality: "Blunt, honest, drinks too much.",
			Motivations: "Finish the Blackfin sloop before hurricane season.",
			DefaultLocation: "astillero_negro",
			Tags:        []string{"craft", "commoner"},
		},
		{
			ID: "npc_coral_priestess", Name: "Sacerdotisa Coral", Role: "tide priestess",
			Appearance:  "Coral rosary; bare feet; algae-green shawl.",
			Personality: "Gentle in public; steel when rites are mocked.",
			Motivations: "Keep Perla Azul neutral in the Crown–Corsair war.",
			Knowledge:   []string{"Tide omens", "Pearl diving rites", "Coven politics (partial)"},
			DefaultLocation: "templo_mareas", FactionID: "coven_tides",
			Tags:        []string{"holy", "sea-witch"},
		},
		{
			ID: "npc_almirante_ribera", Name: "Almirante Ribera", Role: "naval commander",
			Appearance:  "Braid-heavy uniform; powder burns on left cheek.",
			Personality: "By-the-book; despises pirates and witchcraft equally.",
			Motivations: "Sink Valeria Storm's squadron before the next moon.",
			StatBlock:   ptrStatBlock(marineOfficerStatBlock()),
			Disposition: "lawful", FactionID: "crown_armada", DefaultLocation: "cuartel_almirez",
			Tags:        []string{"military", "naval"},
		},
		{
			ID: "npc_cuervo_salazar", Name: "Cuervo Salazar", Role: "pirate captain",
			Appearance:  "Raven-feather tricorn; pistol bandolier.",
			Personality: "Mocking, reckless, superstitious about crows.",
			Motivations: "Claim Puerto Sombrío's docks for the Council.",
			StatBlock:   ptrStatBlock(captainStatBlock()),
			FactionID: "free_corsair_council", DefaultLocation: "taberna_ancla_podrida",
			Tags:        []string{"pirate", "villain"},
		},
		{
			ID: "npc_paco_tabernero", Name: "Paco el Tabernero", Role: "innkeeper",
			Appearance:  "Barrel-chested; apron; missing two fingers.",
			Personality: "Fatherly until you break house rules.",
			Motivations: "Keep the Ancla Podrida neutral ground.",
			SampleDialogue: []string{"No blades drawn inside — that's what the alley's for."},
			DefaultLocation: "taberna_ancla_podrida",
			Tags:        []string{"tavern", "informant"},
		},
		{
			ID: "npc_cartografo_mir", Name: "Tomás Mir", Role: "cartographer",
			Appearance:  "Ink-stained fingers; brass dividers on a chain.",
			Personality: "Obsessive about accuracy.",
			Motivations: "Chart the trench without Crown censorship.",
			Knowledge:   []string{"Hidden reefs", "Ghost Shoals drift", "Trench depth soundings"},
			DefaultLocation: "malecon_faro",
			Tags:        []string{"scholar", "navigation"},
		},
		{
			ID: "npc_isla_marinera", Name: "Isla Cortavientos", Role: "deckhand",
			Appearance:  "Teen sailor; shaved head; knife in boot.",
			Personality: "Quick, curious, fearless to a fault.",
			Motivations: "Earn a berth on Valeria's crew.",
			DefaultLocation: "taberna_ancla_podrida",
			Tags:        []string{"sailor", "commoner"},
		},
		{
			ID: "npc_envoy_sombrio", Name: "Envoy Lucía Mendoza", Role: "customs official",
			Appearance:  "Crown livery; spectacles; ledger always open.",
			Personality: "Corrupt but polite.",
			Motivations: "Maximize bribes while appearing loyal to Almirez.",
			Secrets:     "Keeps a list of smugglers for blackmail.",
			DefaultLocation: "aduana_sombrio", FactionID: "crown_armada",
			Tags:        []string{"civic", "colonial"},
		},
	}
	for _, npc := range npcs {
		addNPC(p, npc)
	}
}

func buildLocationContents(p *worldpack.Pack) {
	contents := []worldpack.LocationContents{
		{
			LocationID: "taberna_ancla_podrida",
			NPCIDs:     []string{"npc_paco_tabernero", "npc_valeria_storm", "npc_cuervo_salazar", "npc_isla_marinera"},
			ItemIDs:    []string{"item_grog", "item_rum_fine"},
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
			LocationID: "faro_tormenta",
			ItemIDs:    []string{"item_spyglass", "item_compass"},
			EncounterTableID: "encounter_open_sea",
		},
	}
	for _, lc := range contents {
		addLocationContents(p, lc)
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
		addEncounterTable(p, t)
	}
}

func buildToolExamples(p *worldpack.Pack) {
	bindToolFromCanonical(p, "get_city", "Puerto Sombrío Overview", "Fetch Puerto Sombrío city record.", "geography", nil,
		[]worldpack.ToolExample{{Title: "Party docks", Input: map[string]any{"city_id": "puerto_sombrio"}, Output: "Districts, population, harbor locations."}})
	bindToolFromCanonical(p, "list_city_locations", "List Sombrío Docks", "Locations in Muelle Viejo.", "geography", nil,
		[]worldpack.ToolExample{{Title: "Find tavern", Input: map[string]any{"city_id": "puerto_sombrio", "district_id": "muelle_viejo"}, Output: "taberna_ancla_podrida, astillero_negro"}})
	bindToolFromCanonical(p, "get_npc", "Meet Capitán Storm", "Load privateer captain NPC.", "population", nil,
		[]worldpack.ToolExample{{Title: "Tavern contact", Input: map[string]any{"npc_id": "npc_valeria_storm"}, Output: "Personality, faction, stat block."}})
	bindToolFromCanonical(p, "roll_encounter_table", "Reef crossing roll", "Roll coral reef table.", "encounters", nil,
		[]worldpack.ToolExample{{Title: "d10 = 7", Input: map[string]any{"table_id": "encounter_reef", "roll": 7}, Output: "1d4+1 reef sharks."}})
	bindToolFromCanonical(p, "search_world", "Search smugglers", "Find smuggler references.", "reference", nil,
		[]worldpack.ToolExample{{Title: "Query smuggler", Input: map[string]any{"query": "contrabandista", "limit": 5}, Output: "npc_garfio_reyes, cala_contrabandistas, item_chart_shoals..."}})
}

func buildOracleScenarios(p *worldpack.Pack) {
	p.OracleGuide.Scenarios = append(p.OracleGuide.Scenarios,
		worldpack.GuideScenario{
			Situation:  "Party docks at Puerto Sombrío",
			UseTools:   []string{"get_city", "list_city_locations", "get_location", "list_location_npcs", "get_lore"},
			Avoid:      []string{"Inventing a new harbor district", "Replacing Taberna del Ancla Podrida with a generic tavern"},
			InventWhen: "Players seek a shop type not listed — invent name but reuse market or dock tags.",
		},
		worldpack.GuideScenario{
			Situation: "Random open-sea voyage day",
			UseTools:  []string{"roll_encounter_table", "get_creature", "describe_travel"},
			Avoid:     []string{"Custom naval encounters when encounter_open_sea exists"},
		},
		worldpack.GuideScenario{
			Situation: "Party enters Ghost Shoals mist",
			UseTools:  []string{"get_region", "roll_encounter_table", "get_lore", "get_creature"},
			Avoid:     []string{"Ignoring curse lore from lore_shoal_bells"},
		},
	)
}

// --- stat block helpers ---

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

// --- local builder helpers (mirror worldpack/profiles/builder.go until worldpack exports them) ---





func addRegion(p *worldpack.Pack, id, name, desc, biome string, tags ...string) {
	p.Regions = append(p.Regions, worldpack.Region{
		ID: id, Name: name, Description: desc, Biome: biome, Tags: tags,
	})
}

func addCity(p *worldpack.Pack, id, name, regionID, desc string, districts []worldpack.District, tags ...string) {
	p.Cities = append(p.Cities, worldpack.City{
		ID: id, Name: name, RegionID: regionID, Description: desc, Districts: districts, Tags: tags,
	})
}

func addDistrict(id, name, desc string, locationIDs []string, tags ...string) worldpack.District {
	return worldpack.District{
		ID: id, Name: name, Description: desc, LocationIDs: locationIDs, Tags: tags,
	}
}

func addLoc(p *worldpack.Pack, loc worldpack.Location) {
	p.Locations = append(p.Locations, loc)
}

func addNPC(p *worldpack.Pack, npc worldpack.WorldNPC) {
	p.NPCs = append(p.NPCs, npc)
}

func addCreatureSavage(p *worldpack.Pack, id, name string, sb domain.StatBlock, habitats []string, encounterNotes, lore string, tags ...string) {
	p.Creatures = append(p.Creatures, worldpack.CreatureEntry{
		ID:             id,
		Name:           name,
		StatBlock:      sb,
		Habitats:       habitats,
		CR:             sb.CR,
		Tags:           tags,
		EncounterNotes: encounterNotes,
		Lore:           lore,
		ToolAdapter:    "lookup_creature",
	})
}

func addItem(p *worldpack.Pack, item worldpack.WorldItem) {
	p.Items = append(p.Items, item)
}

func addFaction(p *worldpack.Pack, id, name, desc, goals string) {
	p.Factions = append(p.Factions, domain.Faction{ID: id, Name: name, Description: desc, Goals: goals})
}

func addLore(p *worldpack.Pack, id, title, content, regionID string, tags ...string) {
	p.Lore = append(p.Lore, worldpack.LoreEntry{ID: id, Title: title, Content: content, RegionID: regionID, Tags: tags})
}

func addMap(p *worldpack.Pack, id, name, kind, path, desc, scale string) {
	p.Maps = append(p.Maps, worldpack.MapRef{ID: id, Name: name, Kind: kind, Path: path, Description: desc, Scale: scale})
}

func addEncounterTable(p *worldpack.Pack, t worldpack.EncounterTable) {
	p.EncounterTables = append(p.EncounterTables, t)
}

func addLocationContents(p *worldpack.Pack, lc worldpack.LocationContents) {
	p.LocationContents = append(p.LocationContents, lc)
}

func bindToolFromCanonical(p *worldpack.Pack, canonicalID, name, description, category string, preconditions []string, examples []worldpack.ToolExample) {
	ct, ok := worldpack.CanonicalByID(canonicalID)
	params := map[string]any{}
	if ok {
		params = ct.Parameters
	}
	raw, _ := json.Marshal(params)
	p.Tools = append(p.Tools, worldpack.ToolBinding{
		CanonicalID:   canonicalID,
		Enabled:       true,
		Name:          name,
		Description:   description,
		Parameters:    raw,
		Category:      category,
		Preconditions: preconditions,
		Examples:      examples,
	})
	if p.Compatibility.ToolMap == nil {
		p.Compatibility.ToolMap = map[string]string{}
	}
	p.Compatibility.ToolMap[name] = canonicalID
}

func linkCityToRegion(p *worldpack.Pack, regionID, cityID string) {
	for i := range p.Regions {
		if p.Regions[i].ID == regionID {
			p.Regions[i].CityIDs = append(p.Regions[i].CityIDs, cityID)
			return
		}
	}
}

func linkLocationToCity(p *worldpack.Pack, cityID, districtID, locationID string) {
	for i := range p.Cities {
		if p.Cities[i].ID != cityID {
			continue
		}
		p.Cities[i].LocationIDs = append(p.Cities[i].LocationIDs, locationID)
		if districtID != "" {
			for j := range p.Cities[i].Districts {
				if p.Cities[i].Districts[j].ID == districtID {
					p.Cities[i].Districts[j].LocationIDs = append(p.Cities[i].Districts[j].LocationIDs, locationID)
				}
			}
		}
		return
	}
}

func linkWildernessLocation(p *worldpack.Pack, regionID, locationID string) {
	for i := range p.Regions {
		if p.Regions[i].ID == regionID {
			p.Regions[i].LocationIDs = append(p.Regions[i].LocationIDs, locationID)
			return
		}
	}
}

func joinStrings(ss []string) string {
	if len(ss) == 0 {
		return ""
	}
	out := ss[0]
	for i := 1; i < len(ss); i++ {
		out += "," + ss[i]
	}
	return out
}
