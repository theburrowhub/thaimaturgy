package caribdus

import (
	"github.com/theburrowhub/thaimaturgy/internal/worldpack"
)

func buildRegions(p *worldpack.Pack) {
	worldpack.AddRegion(p, "open_sea", "Open Sea",
		"Trackless swells between island chains. Squalls rise without warning; hulls groan on long swells.",
		"ocean", "travel", "naval")
	p.Regions[0].TravelNotes = "Navigation checks each day; off-course drift toward Ghost Shoals on failed rolls."
	p.Regions[0].MapID = "map_open_sea"

	worldpack.AddRegion(p, "central_archipelago", "Central Archipelago",
		"The heart of Caribdus — crowded anchorages, reef mazes, and the three great ports.",
		"archipelago", "trade", "civilized")
	p.Regions[1].CityIDs = []string{"puerto_sombrio", "perla_azul", "fuerte_almirez"}
	p.Regions[1].TravelNotes = "Inter-island hops take hours by sloop; reef pilots charge 5 gp per passage."
	p.Regions[1].MapID = "map_central_archipelago"

	worldpack.AddRegion(p, "ghost_shoals", "Ghost Shoals",
		"Mist-wrapped shallows where compasses spin and drowned bells toll at dusk.",
		"mist", "undead", "supernatural")
	p.Regions[2].TravelNotes = "Visibility rarely exceeds 100 ft.; sea-witch pacts rumored to calm the fog."
	p.Regions[2].MapID = "map_ghost_shoals"

	worldpack.AddRegion(p, "deep_trench", "Deep Trench",
		"A black scar in the ocean floor. Bioluminescent hunters and Deluge ruins lurk below the thermocline.",
		"deep_ocean", "horror", "ruins")
	p.Regions[3].TravelNotes = "Surface ships avoid the trench; diving bells and enchanted lungs are mandatory."
	p.Regions[3].MapID = "map_deep_trench"
}

func buildMaps(p *worldpack.Pack) {
	worldpack.AddMap(p, "map_central_archipelago", "Central Archipelago Chart", "regional",
		"maps/central_archipelago.png", "Reef channels, trade winds, and the three port cities marked with anchor symbols.", "1 hex = 8 nautical miles")
	worldpack.AddMap(p, "map_puerto_sombrio", "Puerto Sombrío Harbor", "city",
		"maps/puerto_sombrio.png", "Colonial walls, black-market docks, and the shipyard cranes.", "1 square = 50 ft")
	worldpack.AddMap(p, "map_perla_azul", "Perla Azul Lagoon", "city",
		"maps/perla_azul.png", "Pearl market, tide temple, and coral breakwater.", "1 square = 50 ft")
	worldpack.AddMap(p, "map_fuerte_almirez", "Fuerte Almirez Citadel", "city",
		"maps/fuerte_almirez.png", "Naval fort, governor's plaza, and ironclad shipyard.", "1 square = 50 ft")
	worldpack.AddMap(p, "map_ghost_shoals", "Ghost Shoals Mist Chart", "regional",
		"maps/ghost_shoals.png", "Shallow wrecks, witch huts, and tide rips hatched in gray.", "1 hex = 4 nautical miles")
}
