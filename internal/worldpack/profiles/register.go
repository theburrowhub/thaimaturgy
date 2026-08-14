package profiles

import "github.com/theburrowhub/thaimaturgy/internal/worldpack"

func init() {
	worldpack.RegisterBuiltin("dnd5e_shattered_vale", DnD5eShatteredVale)
	worldpack.RegisterBuiltinAlias("dnd5e", "dnd5e_shattered_vale")
	worldpack.RegisterBuiltinAlias("shattered_vale", "dnd5e_shattered_vale")
}
