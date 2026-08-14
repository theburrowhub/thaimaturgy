package worlds

import (
	"github.com/theburrowhub/thaimaturgy/internal/worldpack"
	"github.com/theburrowhub/thaimaturgy/internal/worldpack/worlds/caribdus"
	"github.com/theburrowhub/thaimaturgy/internal/worldpack/worlds/mistfall_coast"
	"github.com/theburrowhub/thaimaturgy/internal/worldpack/worlds/shattered_vale"
)

func init() {
	worldpack.RegisterWorld("shattered_vale", shattered_vale.Build)
	worldpack.RegisterWorldAlias("dnd5e_shattered_vale", "shattered_vale")
	worldpack.RegisterWorldAlias("dnd5e", "shattered_vale")

	worldpack.RegisterWorld("caribdus", caribdus.Build)
	worldpack.RegisterWorldAlias("50_fathoms", "caribdus")
	worldpack.RegisterWorldAlias("50_brazas", "caribdus")

	worldpack.RegisterWorld("mistfall_coast", mistfall_coast.Build)
	worldpack.RegisterWorldAlias("aldo", "mistfall_coast")
}
