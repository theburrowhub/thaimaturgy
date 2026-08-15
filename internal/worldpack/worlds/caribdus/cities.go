package caribdus

import (
	"github.com/theburrowhub/thaimaturgy/internal/worldpack"
)

func buildCitiesAndLocations(p *worldpack.Pack) {
	// --- Puerto Sombrío ---
	sombrioDistricts := []worldpack.District{
		worldpack.AddDistrict("muelle_viejo", "Muelle Viejo", "Rotting piers and tariff shacks; first sight of Caribdus for many crews.", nil, "docks", "trade"),
		worldpack.AddDistrict("barrio_velas", "Barrio de las Velas", "Sail-loft alleys, chandlers, and boarding houses.", nil, "maritime", "urban"),
		worldpack.AddDistrict("muralla_colonial", "Muralla Colonial", "Stone walls, customs house, and the governor's shadow.", nil, "colonial", "law"),
	}
	worldpack.AddCity(p, "puerto_sombrio", "Puerto Sombrío", "central_archipelago",
		"Grim port of fifteen thousand souls where Crown tariffs meet pirate gold. Smoke and tar hang over every dawn.",
		sombrioDistricts, "port", "colonial", "smuggling")
	p.Cities[0].Population = "~15,000"
	p.Cities[0].Government = "Colonial governor with a bribed council"
	p.Cities[0].MapID = "map_puerto_sombrio"
	worldpack.LinkCityToRegion(p, "central_archipelago", "puerto_sombrio")

	worldpack.AddLocation(p, worldpack.Location{
		ID: "taberna_ancla_podrida", Name: "Taberna del Ancla Podrida", Kind: "tavern",
		CityID: "puerto_sombrio", DistrictID: "muelle_viejo", RegionID: "central_archipelago",
		Description: "Three-story flophouse favored by bosuns and deserters. Rooms 4 sp; grog 2 cp; stew questionable.",
		ReadAloud:   "A barnacle-crusted anchor hangs above the door. Inside, dice clatter and a concertina wheezes through pipe smoke.",
		DMNotes:     "Tabernero Paco hears every dock rumor. Hidden hatch in the cellar leads toward smuggler tunnels.",
		Tags:        []string{"tavern", "rest", "rumors"},
		Connections: []string{"astillero_negro", "mercado_especias"},
	})
	worldpack.AddLocation(p, worldpack.Location{
		ID: "astillero_negro", Name: "Astillero Negro", Kind: "shipyard",
		CityID: "puerto_sombrio", DistrictID: "muelle_viejo", RegionID: "central_archipelago",
		Description: "Black-tarred slips where hulls are patched between raids. Contramaestre Toro runs the night shift.",
		ReadAloud:   "Hammer on copper rivets rings across oily water. Cranes swing masts like matchsticks.",
		DMNotes:     "Stolen naval timber stored under tarpaulin 7. Athletics DC 14 to slip aboard a refitting sloop.",
		Tags:        []string{"shipyard", "craft", "travel"},
		Connections: []string{"taberna_ancla_podrida", "cala_contrabandistas"},
	})
	worldpack.AddLocation(p, worldpack.Location{
		ID: "mercado_especias", Name: "Mercado de Especias", Kind: "market",
		CityID: "puerto_sombrio", DistrictID: "barrio_velas", RegionID: "central_archipelago",
		Description: "Canvas stalls selling pepper, rum, compass needles, and questionable charms.",
		ReadAloud:   "Spice heat and tar compete in the air. Hawkers bark prices in three languages.",
		Tags:        []string{"market", "shopping", "social"},
		Connections: []string{"taberna_ancla_podrida", "aduana_sombrio"},
	})
	worldpack.AddLocation(p, worldpack.Location{
		ID: "aduana_sombrio", Name: "Aduana Colonial", Kind: "civic",
		CityID: "puerto_sombrio", DistrictID: "muralla_colonial", RegionID: "central_archipelago",
		Description: "Customs house where Crown clerks stamp manifests and squeeze bribes.",
		ReadAloud:   "Wax seals and iron grilles. A clerk's quill scratches like a rat in the wall.",
		DMNotes:     "Governor's envoy visits on high tide. Hidden ledger of smuggler payoffs in the vault.",
		Tags:        []string{"civic", "law", "colonial"},
		Connections: []string{"mercado_especias", "fortin_colonial"},
	})
	worldpack.AddLocation(p, worldpack.Location{
		ID: "fortin_colonial", Name: "Fortín Colonial", Kind: "fort",
		CityID: "puerto_sombrio", DistrictID: "muralla_colonial", RegionID: "central_archipelago",
		Description: "Cannon battery guarding the harbor mouth. Powder magazine below the parapet.",
		ReadAloud:   "Salt-crusted cannon stare seaward. Red-coated marines drill in the yard.",
		Tags:        []string{"military", "fort"},
		Connections: []string{"aduana_sombrio"},
	})

	// --- Perla Azul ---
	perlaDistricts := []worldpack.District{
		worldpack.AddDistrict("mercado_perlas", "Mercado de Perlas", "Open-air auction pits and diver guild halls.", nil, "trade", "pearl"),
		worldpack.AddDistrict("malecon_dorado", "Malecón Dorado", "Wealthy villas on the lagoon rim.", nil, "noble", "coast"),
		worldpack.AddDistrict("barrio_pescador", "Barrio Pescador", "Net menders, shrines, and cheap ceviche stalls.", nil, "fishing", "urban"),
	}
	worldpack.AddCity(p, "perla_azul", "Perla Azul", "central_archipelago",
		"Lagoon city built on coral pilings. Pearl trade and tide worship keep the peace — barely.",
		perlaDistricts, "port", "trade", "temple")
	p.Cities[1].Population = "~9,000"
	p.Cities[1].Government = "Pearl Council of master divers and temple elders"
	p.Cities[1].MapID = "map_perla_azul"
	worldpack.LinkCityToRegion(p, "central_archipelago", "perla_azul")

	worldpack.AddLocation(p, worldpack.Location{
		ID: "taberna_perla", Name: "Taberna de la Perla", Kind: "tavern",
		CityID: "perla_azul", DistrictID: "mercado_perlas", RegionID: "central_archipelago",
		Description: "Diver's tavern with a pearl-inlaid bar. Fresh catch served at dawn.",
		ReadAloud:   "Mother-of-pearl gleams beneath tankards. Diver tattoos flash in lamplight.",
		Tags:        []string{"tavern", "rest"},
		Connections: []string{"casa_cambio_perla", "templo_mareas"},
	})
	worldpack.AddLocation(p, worldpack.Location{
		ID: "casa_cambio_perla", Name: "Casa de Cambio del Arrecife", Kind: "shop",
		CityID: "perla_azul", DistrictID: "mercado_perlas", RegionID: "central_archipelago",
		Description: "Money-changer and fence for unmarked pearls. Heavy shutters, heavier guards.",
		ReadAloud:   "Scales ping. A grille slides open just wide enough for hands and coin.",
		Tags:        []string{"shop", "finance"},
		Connections: []string{"taberna_perla"},
	})
	worldpack.AddLocation(p, worldpack.Location{
		ID: "templo_mareas", Name: "Templo de las Mareas", Kind: "temple",
		CityID: "perla_azul", DistrictID: "malecon_dorado", RegionID: "central_archipelago",
		Description: "Open-air shrine where tide priests read omens in shell patterns.",
		ReadAloud:   "Conch horns echo at dusk. Incense of seaweed and myrrh drifts over the lagoon.",
		DMNotes:     "Sacerdotisa Coral can bless water-breathing for a tithe.",
		Tags:        []string{"temple", "holy", "sea-witch"},
		Connections: []string{"taberna_perla", "malecon_faro"},
	})
	worldpack.AddLocation(p, worldpack.Location{
		ID: "malecon_faro", Name: "Faro del Malecón", Kind: "lighthouse",
		CityID: "perla_azul", DistrictID: "malecon_dorado", RegionID: "central_archipelago",
		Description: "White lighthouse guiding lagoon traffic. Cartographer's workshop in the base.",
		ReadAloud:   "A beam sweeps over coral breakers. Charts rustle in a salt-stained room below.",
		Tags:        []string{"navigation", "landmark"},
		Connections: []string{"templo_mareas"},
	})

	// --- Fuerte Almirez ---
	almirezDistricts := []worldpack.District{
		worldpack.AddDistrict("plaza_armas", "Plaza de Armas", "Parade ground and governor's palace.", nil, "civic", "military"),
		worldpack.AddDistrict("astilleros_navales", "Astilleros Navales", "Crown shipyard and rope walks.", nil, "shipyard", "naval"),
		worldpack.AddDistrict("cuartel_naval", "Cuartel Naval", "Barracks, powder store, and naval court.", nil, "military", "law"),
	}
	worldpack.AddCity(p, "fuerte_almirez", "Fuerte Almirez", "central_archipelago",
		"Stone citadel and naval anchor of the Crown Armada. Discipline, cannon, and colonial ambition.",
		almirezDistricts, "fortress", "naval", "colonial")
	p.Cities[2].Population = "~12,000"
	p.Cities[2].Government = "Governor-General Mateo Almirez"
	p.Cities[2].MapID = "map_fuerte_almirez"
	worldpack.LinkCityToRegion(p, "central_archipelago", "fuerte_almirez")

	worldpack.AddLocation(p, worldpack.Location{
		ID: "palacio_gobernador", Name: "Palacio del Gobernador", Kind: "civic",
		CityID: "fuerte_almirez", DistrictID: "plaza_armas", RegionID: "central_archipelago",
		Description: "Marble-faced palace where colonial edicts become law.",
		ReadAloud:   "Guards in blue coats flank a gate of black iron. Fanfare is optional; taxes are not.",
		Tags:        []string{"civic", "noble", "quests"},
		Connections: []string{"cuartel_almirez", "astillero_crown"},
	})
	worldpack.AddLocation(p, worldpack.Location{
		ID: "cuartel_almirez", Name: "Cuartel del Gobernador", Kind: "barracks",
		CityID: "fuerte_almirez", DistrictID: "cuartel_naval", RegionID: "central_archipelago",
		Description: "Naval barracks housing two hundred marines. Armory issues weapons by writ only.",
		ReadAloud:   "Boot heels crack on stone. The smell of gun oil is stronger than the sea.",
		Tags:        []string{"military", "law"},
		Connections: []string{"palacio_gobernador", "cantina_mosquito"},
	})
	worldpack.AddLocation(p, worldpack.Location{
		ID: "astillero_crown", Name: "Astillero de la Corona", Kind: "shipyard",
		CityID: "fuerte_almirez", DistrictID: "astilleros_navales", RegionID: "central_archipelago",
		Description: "Crown shipyard building frigates and powder hulks.",
		ReadAloud:   "Sawdust snows on the quay. A frigate skeleton waits for its copper sheathing.",
		Tags:        []string{"shipyard", "naval"},
		Connections: []string{"palacio_gobernador"},
	})
	worldpack.AddLocation(p, worldpack.Location{
		ID: "cantina_mosquito", Name: "Cantina del Mosquito", Kind: "tavern",
		CityID: "fuerte_almirez", DistrictID: "cuartel_naval", RegionID: "central_archipelago",
		Description: "Off-duty naval tavern. Darts, dice, and discreet duels in the alley.",
		ReadAloud:   "A mosquito is nailed above the door — wings spread, iron pin through the thorax.",
		Tags:        []string{"tavern", "social"},
		Connections: []string{"cuartel_almirez"},
	})

	// --- Wilderness & sea locations ---
	worldpack.AddLocation(p, worldpack.Location{
		ID: "arrecife_coral", Name: "Arrecife de Coral Sangriento", Kind: "wilderness",
		RegionID:    "central_archipelago",
		Description: "Crimson coral maze that tears hulls. Sharks patrol the outer rim.",
		ReadAloud:   "Breakers hiss over coral teeth. The lagoon inside is eerily calm.",
		Tags:        []string{"reef", "wilderness", "danger"},
		Connections: []string{"cala_contrabandistas"},
	})
	worldpack.LinkWildernessLocation(p, "central_archipelago", "arrecife_coral")

	worldpack.AddLocation(p, worldpack.Location{
		ID: "cala_contrabandistas", Name: "Cala de los Contrabandistas", Kind: "wilderness",
		RegionID:    "central_archipelago",
		Description: "Hidden cove with camouflaged nets and a pulley hoist into the cliffs.",
		ReadAloud:   "Tarps mimic rock from the sea. A whistle answers your approach — friend or foe?",
		DMNotes:     "Garfio Reyes uses this cove. Smuggler lookouts Perception +4.",
		Tags:        []string{"smuggling", "cove", "wilderness"},
		Connections: []string{"astillero_negro", "arrecife_coral"},
	})
	worldpack.LinkWildernessLocation(p, "central_archipelago", "cala_contrabandistas")

	worldpack.AddLocation(p, worldpack.Location{
		ID: "choza_bruja_mar", Name: "Choza de la Bruja del Mar", Kind: "wilderness",
		RegionID:    "ghost_shoals",
		Description: "Stilt hut on a dead mangrove. Shell wind chimes and cauldron smoke.",
		ReadAloud:   "A hut squats on stilts above black water. Chimes clack though there is no wind.",
		DMNotes:     "Marisela la Bruma trades storms for favors. Coven audience by invitation only.",
		Tags:        []string{"sea-witch", "wilderness", "magic"},
	})
	worldpack.LinkWildernessLocation(p, "ghost_shoals", "choza_bruja_mar")

	worldpack.AddLocation(p, worldpack.Location{
		ID: "naufragio_espectral", Name: "Naufragio Espectral", Kind: "wilderness",
		RegionID:    "ghost_shoals",
		Description: "Ghostly galleon fused with the reef. Bells toll at midnight.",
		ReadAloud:   "Mist parts around a half-sunk hull. Rigging moves with no crew aboard.",
		Tags:        []string{"undead", "wreck", "wilderness"},
	})
	worldpack.LinkWildernessLocation(p, "ghost_shoals", "naufragio_espectral")

	worldpack.AddLocation(p, worldpack.Location{
		ID: "faro_tormenta", Name: "Faro de la Tormenta", Kind: "lighthouse",
		RegionID:    "open_sea",
		Description: "Storm-lashed lighthouse on a bare rock. Keeper hasn't been paid in years.",
		ReadAloud:   "Lightning fractures the sky. The beacon still turns — who fuels it?",
		Tags:        []string{"lighthouse", "open_sea", "mystery"},
	})
	worldpack.LinkWildernessLocation(p, "open_sea", "faro_tormenta")

	worldpack.AddLocation(p, worldpack.Location{
		ID: "templo_sumergido", Name: "Templo Sumergido del Abismo", Kind: "dungeon",
		RegionID:    "deep_trench",
		Description: "Deluge-era temple half buried in the trench wall. Air pockets hold stale breath.",
		ReadAloud:   "Stone angels stare through green dark. Your lamp throws shadows that move wrong.",
		Tags:        []string{"ruins", "deep", "dungeon"},
	})
	worldpack.LinkWildernessLocation(p, "deep_trench", "templo_sumergido")

	worldpack.AddLocation(p, worldpack.Location{
		ID: "fosa_abisal", Name: "Fosa Abisal de las Luces", Kind: "wilderness",
		RegionID:    "deep_trench",
		Description: "Bioluminescent vent field. Pressure and predators end careless dives.",
		ReadAloud:   "Cold light blooms in the black. Something vast shifts below your keel.",
		Tags:        []string{"deep", "horror", "wilderness"},
	})
	worldpack.LinkWildernessLocation(p, "deep_trench", "fosa_abisal")

	// Link all city locations to districts
	for _, loc := range p.Locations {
		if loc.CityID != "" {
			worldpack.LinkLocationToCity(p, loc.CityID, loc.DistrictID, loc.ID)
		}
	}
}
