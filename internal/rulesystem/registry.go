package rulesystem

import "fmt"

var builtinFactories = map[string]func() *Pack{}

// RegisterBuiltin registers a built-in pack factory by ID.
func RegisterBuiltin(id string, factory func() *Pack) {
	builtinFactories[id] = factory
}

// RegisterBuiltinAlias registers an alternate ID for an existing factory.
func RegisterBuiltinAlias(alias, id string) {
	factory, ok := builtinFactories[id]
	if !ok {
		return
	}
	builtinFactories[alias] = factory
}

// Builtin returns a built-in pack by ID.
func Builtin(id string) (*Pack, error) {
	factory, ok := builtinFactories[id]
	if !ok {
		return nil, fmt.Errorf("unknown built-in pack %q", id)
	}
	return factory(), nil
}

// BuiltinIDs returns registered built-in pack identifiers (canonical IDs only).
func BuiltinIDs() []string {
	return []string{"dnd5e", "d100", "savage_worlds"}
}

// ListBuiltins returns all built-in packs.
func ListBuiltins() []*Pack {
	out := make([]*Pack, 0, len(BuiltinIDs()))
	for _, id := range BuiltinIDs() {
		p, err := Builtin(id)
		if err == nil && p != nil {
			out = append(out, p)
		}
	}
	return out
}
