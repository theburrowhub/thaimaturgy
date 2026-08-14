package worldpack

import "fmt"

var worldFactories = map[string]func() *Pack{}

func RegisterWorld(id string, factory func() *Pack) { worldFactories[id] = factory }

func RegisterWorldAlias(alias, id string) {
	if f, ok := worldFactories[id]; ok {
		worldFactories[alias] = f
	}
}

func RegisterBuiltin(id string, factory func() *Pack)     { RegisterWorld(id, factory) }
func RegisterBuiltinAlias(alias, id string)             { RegisterWorldAlias(alias, id) }

func Builtin(id string) (*Pack, error) {
	f, ok := worldFactories[id]
	if !ok {
		return nil, fmt.Errorf("unknown world %q (use worldpack-gen -list)", id)
	}
	return f(), nil
}

var canonicalWorldIDs = []string{"shattered_vale", "caribdus", "mistfall_coast"}

func BuiltinIDs() []string { return append([]string(nil), canonicalWorldIDs...) }

func ListBuiltins() []*Pack {
	out := make([]*Pack, 0, len(BuiltinIDs()))
	for _, id := range BuiltinIDs() {
		if p, err := Builtin(id); err == nil && p != nil {
			out = append(out, p)
		}
	}
	return out
}
