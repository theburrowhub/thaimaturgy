package rulesystem

import "fmt"

// BuiltinIDs are the shipped starter templates.
var BuiltinIDs = []string{"dnd5e", "d100", "savage_worlds"}

// Builtin returns a validated starter pack by id.
func Builtin(id string) (*Pack, error) {
	switch id {
	case "dnd5e":
		return validateClone(DnD5e())
	case "d100":
		return validateClone(D100())
	case "savage_worlds":
		return validateClone(SavageWorlds())
	default:
		return nil, fmt.Errorf("unknown builtin %q (choices: %v)", id, BuiltinIDs)
	}
}

func validateClone(p *Pack) (*Pack, error) {
	if err := Validate(p); err != nil {
		return nil, err
	}
	return p, nil
}

// ListBuiltins returns shallow copies of all built-in packs.
func ListBuiltins() ([]*Pack, error) {
	out := make([]*Pack, 0, len(BuiltinIDs))
	for _, id := range BuiltinIDs {
		p, err := Builtin(id)
		if err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	return out, nil
}
