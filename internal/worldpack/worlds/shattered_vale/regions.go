package shattered_vale

import (
	"github.com/theburrowhub/thaimaturgy/internal/worldpack"
)

func buildRegions(p *worldpack.Pack) {
	worldpack.AddRegion(p, "northern_marches", "Northern Marches",
		"Rolling moorland and broken keeps north of the vale. Bandits and undead haunt the old imperial roads.",
		"grassland", "cold", "ruins", "bandits")
	p.Regions[0].CityIDs = []string{"thornwall"}
	p.Regions[0].TravelNotes = "Two days from Millhaven by road; winter snows can close the pass for weeks."
	p.Regions[0].MapID = "map_northern_marches"

	worldpack.AddRegion(p, "sunlit_coast", "Sunlit Coast",
		"Cliff-lined shores and fishing villages where smugglers and sahuagin rumors mix with salt spray.",
		"coastal", "trade", "smuggling")
	p.Regions[1].CityIDs = []string{"millhaven"}
	p.Regions[1].TravelNotes = "Coastal road is well patrolled near Millhaven; ship travel to Ironhold takes half a day."
	p.Regions[1].MapID = "map_sunlit_coast"

	worldpack.AddRegion(p, "whisperwood", "Whisperwood",
		"An ancient forest whose trees murmur at dusk. Fey trails and goblin warrens lurk beneath the canopy.",
		"forest", "fey", "goblin")
	p.Regions[2].TravelNotes = "Travelers should not leave the King's Road after dark."
	p.Regions[2].MapID = "map_whisperwood"

	worldpack.AddRegion(p, "ironspine_mountains", "Ironspine Mountains",
		"Jagged peaks rich in ore. Hobgoblin legions drill in hidden valleys; ogres block high passes.",
		"mountain", "mining", "hobgoblin")
	p.Regions[3].CityIDs = []string{"ironhold"}
	p.Regions[3].TravelNotes = "Mountain passes require Survival DC 12 in winter; avalanches are common."
	p.Regions[3].MapID = "map_ironspine"

	worldpack.AddRegion(p, "undercrypt", "Undercrypt",
		"A buried necropolis opened by the Shattering. Cultists and undead swell its halls.",
		"underground", "undead", "dungeon")
	p.Regions[4].TravelNotes = "No safe overland route; entrances are scattered sinkholes and collapsed temples."
	p.Regions[4].MapID = "map_undercrypt"
}

func buildMaps(p *worldpack.Pack) {
	worldpack.AddMap(p, "map_northern_marches", "Northern Marches Overview", "regional",
		"maps/northern_marches.png", "Moor roads, ruined towers, Thornwall marked at the frontier.", "1 hex = 6 miles")
	worldpack.AddMap(p, "map_sunlit_coast", "Sunlit Coast Chart", "regional",
		"maps/sunlit_coast.png", "River delta, Millhaven harbor, cliff paths.", "1 hex = 4 miles")
	worldpack.AddMap(p, "map_whisperwood", "Whisperwood Trails", "regional",
		"maps/whisperwood.png", "King's Road, grove shrines, goblin territory hatched in green.", "1 hex = 4 miles")
	worldpack.AddMap(p, "map_ironspine", "Ironspine Passes", "regional",
		"maps/ironspine.png", "Fortress Ironhold, mining camps, hobgoblin markers.", "1 hex = 5 miles")
	worldpack.AddMap(p, "map_undercrypt", "Undercrypt Levels", "dungeon",
		"maps/undercrypt.png", "Collapsed temple entrance and three known sub-levels.", "1 square = 10 ft")
	worldpack.AddMap(p, "map_millhaven", "Millhaven City Map", "city",
		"maps/millhaven.png", "Districts: Harbor, Market, Temple Hill, Garrison, Undercroft.", "1 square = 100 ft")
	worldpack.AddMap(p, "map_ironhold", "Ironhold Fortress", "city",
		"maps/ironhold.png", "Keep, smith quarter, training yard, sally ports.", "1 square = 50 ft")
}
