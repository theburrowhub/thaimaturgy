package ruleskit

import (
	_ "embed"

	"github.com/theburrowhub/thaimaturgy/internal/jsonstrict"
	core "github.com/theburrowhub/thaimaturgy/internal/rules"
)

//go:embed engine.go
var engineSource string

//go:embed helpers.go
var helpersSource string

//go:embed source.go
var identitySource string

// NewArtifact builds a source-attested built-in package artifact. packageSource
// must be the embedded production source of the concrete reference package.
func NewArtifact(id, name, description, version, packageSource string, capabilities []string) (core.Artifact, error) {
	manifest := core.Manifest{
		ID:              id,
		Name:            name,
		Description:     description,
		Version:         version,
		ProtocolVersion: core.ProtocolVersion,
		Runtime:         core.Runtime{Kind: core.RuntimeBuiltin},
		Capabilities:    append([]string(nil), capabilities...),
	}
	return core.NewBuiltinArtifact(manifest, map[string]string{
		"package/" + id + "/ruleset.go": packageSource,
		"shared/ruleskit/engine.go":     engineSource,
		"shared/ruleskit/helpers.go":    helpersSource,
		"shared/ruleskit/source.go":     identitySource,
		"shared/jsonstrict":             jsonstrict.SourceIdentity(),
	})
}
