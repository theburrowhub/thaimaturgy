package mistfall_coast

import (
	"github.com/theburrowhub/thaimaturgy/internal/worldpack"
)

func buildCitiesAndLocations(p *worldpack.Pack) {
	hpDistricts := []worldpack.District{
		worldpack.AddDistrict("docks", "North Docks", "Tar smell, net menders, customs sheds.", nil, "maritime"),
		worldpack.AddDistrict("promenade", "Gaslight Promenade", "Hotels, reform clubs, electric lamps.", nil, "wealthy"),
		worldpack.AddDistrict("fogward", "Fogward Slums", "Boarding houses, gin shops, whispered deals.", nil, "poor"),
	}
	worldpack.AddCity(p, "harrowport", "Harrowport", "harrow_bay", "Provincial port of forty thousand souls and one too many secrets.", hpDistricts, "port", "hub")
	p.Cities[0].MapID = "map_harrowport"
	p.Cities[0].Population = "~40,000"
	p.Cities[0].Government = "Harbor Board + elected mayor"

	bfDistricts := []worldpack.District{
		worldpack.AddDistrict("station", "Railway Quarter", "Steam terminus and coal yards.", nil, "trade"),
		worldpack.AddDistrict("square", "Council Square", "Courthouse, papers, respectable inns.", nil, "civic"),
	}
	worldpack.AddCity(p, "brackenford", "Brackenford", "blackwood_hills", "Inland market town where landowners slow every reform bill.", bfDistricts, "inland")

	locs := []worldpack.Location{
		{ID: "harrow_lighthouse", Name: "Harrow Lighthouse", Kind: "landmark", CityID: "harrowport", DistrictID: "docks", RegionID: "harrow_bay",
			Description: "Beam cuts fog nightly; keepers rotate weekly.", Tags: []string{"lighthouse", "cult"}},
		{ID: "constabulary_hq", Name: "Constabulary Headquarters", Kind: "civic", CityID: "harrowport", DistrictID: "promenade", RegionID: "harrow_bay",
			Description: "Gaslit offices; Inspector Vale's domain.", Tags: []string{"law"}},
		{ID: "the_salt_ledger", Name: "The Salt Ledger (newspaper)", Kind: "shop", CityID: "harrowport", DistrictID: "promenade", RegionID: "harrow_bay",
			Description: "Reform paper probing disappearances.", Tags: []string{"investigation"}},
		{ID: "anchor_and_needle", Name: "Anchor & Needle", Kind: "tavern", CityID: "harrowport", DistrictID: "docks", RegionID: "harrow_bay",
			Description: "Sailors' tavern; Mist Runners contacts.", Tags: []string{"social", "smuggling"}},
		{ID: "fogward_chapel", Name: "Fogward Chapel of St. Elian", Kind: "temple", CityID: "harrowport", DistrictID: "fogward", RegionID: "harrow_bay",
			Description: "Charity kitchen; Father Morse hears confessions.", Tags: []string{"faith"}},
		{ID: "customs_house", Name: "Customs House", Kind: "civic", CityID: "harrowport", DistrictID: "docks", RegionID: "harrow_bay",
			Description: "Tariffs enforced with rifles.", Tags: []string{"law", "trade"}},
		{ID: "bracken_inn", Name: "The Moor Cock Inn", Kind: "tavern", CityID: "brackenford", DistrictID: "square", RegionID: "blackwood_hills",
			Description: "Coach stop; councilors drink upstairs.", Tags: []string{"social"}},
		{ID: "standing_stones", Name: "Blackwood Standing Stones", Kind: "wilderness", RegionID: "blackwood_hills",
			Description: "Pre-Christian circle; locals avoid at new moon.", Tags: []string{"occult", "ancient"}},
		{ID: "sealed_mine_shaft", Name: "Sealed Mine Shaft 7", Kind: "dungeon", RegionID: "blackwood_hills",
			Description: "Collapsed tin mine; bricked after 1889 deaths.", Tags: []string{"horror"}},
		{ID: "wreck_black_lantern", Name: "Wreck of the Black Lantern", Kind: "wilderness", RegionID: "salt_flats",
			Description: "Ribbed hull exposed at low tide; lanterns seen at night.", Tags: []string{"undead", "cult"}},
		{ID: "tidal_shrine", Name: "Tidal Shrine", Kind: "temple", RegionID: "salt_flats",
			Description: "Barnacled altar below high-water mark.", Tags: []string{"cult"}},
	}
	for _, loc := range locs {
		worldpack.AddLocation(p, loc)
	}
}
