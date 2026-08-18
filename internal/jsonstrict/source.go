package jsonstrict

import _ "embed"

// These exact production sources are included in built-in rules artifact
// identities because strict decoding is part of their observable behavior.

//go:embed decode.go
var decoderSource string

//go:embed source.go
var identitySource string

// SourceIdentity returns the embedded production source behind Decode.
func SourceIdentity() string {
	return "jsonstrict/decode.go\x00" + decoderSource + "\x00jsonstrict/source.go\x00" + identitySource
}
