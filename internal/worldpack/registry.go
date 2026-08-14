package worldpack

import "fmt"

var builtinFactories = map[string]func() *Pack{}

// RegisterBuiltin registers a built-in world pack factory by ID.
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

// Builtin returns a built-in world pack by ID.
func Builtin(id string) (*Pack, error) {
	factory, ok := builtinFactories[id]
	if !ok {
		return nil, fmt.Errorf("unknown built-in world pack %q", id)
	}
	return factory(), nil
}

// canonicalBuiltinIDs lists primary template IDs (not aliases).
var canonicalBuiltinIDs = []string{"dnd5e_shattered_vale"}

// BuiltinIDs returns registered canonical built-in pack identifiers.
func BuiltinIDs() []string {
	return append([]string(nil), canonicalBuiltinIDs...)
}

// ListBuiltins returns all canonical built-in packs.
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

// AllRegisteredIDs returns every registered ID including aliases.
func AllRegisteredIDs() []string {
	out := make([]string, 0, len(builtinFactories))
	for id := range builtinFactories {
		out = append(out, id)
	}
	for i := 1; i < len(out); i++ {
		for j := i; j > 0 && out[j-1] > out[j]; j-- {
			out[j-1], out[j] = out[j], out[j-1]
		}
	}
	return out
}
