package profiles

import "github.com/theburrowhub/thaimaturgy/internal/rulesystem"

func init() {
	rulesystem.RegisterBuiltin("dnd5e", DnD5e)
	rulesystem.RegisterBuiltinAlias("dnd-5e", "dnd5e")
	rulesystem.RegisterBuiltinAlias("dnd_5e", "dnd5e")

	rulesystem.RegisterBuiltin("d100", D100)
	rulesystem.RegisterBuiltinAlias("brp", "d100")
	rulesystem.RegisterBuiltinAlias("call_of_cthulhu", "d100")

	rulesystem.RegisterBuiltin("savage_worlds", SavageWorlds)
	rulesystem.RegisterBuiltinAlias("swade", "savage_worlds")
	rulesystem.RegisterBuiltinAlias("savage-worlds", "savage_worlds")
}
