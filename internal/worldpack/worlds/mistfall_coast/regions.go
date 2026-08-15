package mistfall_coast

import (
	"github.com/theburrowhub/thaimaturgy/internal/worldpack"
)

func buildRegions(p *worldpack.Pack) {
	worldpack.AddRegion(p, "harrow_bay", "Harrow Bay", "Salt fog, busy docks, gaslit hills patrolled by paired constables.", "coastal", "urban", "fog")
	p.Regions[0].CityIDs = []string{"harrowport"}
	p.Regions[0].TravelNotes = "Steam packet to Brackenford: 3 hours; fog delays night crossings."

	worldpack.AddRegion(p, "blackwood_hills", "Blackwood Hills", "Peat moors, pine ridges, standing stones and sealed mines.", "highland", "wilderness")
	p.Regions[1].CityIDs = []string{"brackenford"}
	p.Regions[1].LocationIDs = []string{"standing_stones", "sealed_mine_shaft"}
	p.Regions[1].TravelNotes = "Coaches by day; Spot Hidden to avoid bog hazards at dusk."

	worldpack.AddRegion(p, "salt_flats", "Salt Flats", "Tidal flats at low tide — wreck ribs, quicksand, glassy pools.", "coastal", "tidal")
	p.Regions[2].LocationIDs = []string{"wreck_black_lantern", "tidal_shrine"}
	p.Regions[2].TravelNotes = "Cross with a guide; tide tables mandatory."
}

func buildMaps(p *worldpack.Pack) {
	worldpack.AddMap(p, "map_harrowport", "Harrowport Chart", "city", "maps/harrowport.png", "Docks, Gaslight Promenade, Constabulary Quarter, Fogward Slums.", "1 sq = 200 ft")
	worldpack.AddMap(p, "map_mistfall", "Mistfall Regional", "region", "maps/mistfall_coast.png", "Harrow Bay, Blackwood Hills, Salt Flats.", "1 hex = 4 mi")
}
