package diceexpr

import _ "embed"

//go:embed expression.go
var expressionSource string

//go:embed source.go
var identitySource string

// SourceIdentity returns the embedded production source behind Parse.
func SourceIdentity() string {
	return "diceexpr/expression.go\x00" + expressionSource + "\x00diceexpr/source.go\x00" + identitySource
}
