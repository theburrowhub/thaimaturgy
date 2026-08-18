package dnd5e

import (
	"embed"
	"fmt"

	"github.com/theburrowhub/thaimaturgy/internal/diceexpr"
	"github.com/theburrowhub/thaimaturgy/internal/jsonstrict"
	core "github.com/theburrowhub/thaimaturgy/internal/rules"
)

//go:embed actions.go codec.go manifest.go results.go ruleset.go source.go state.go
var builtinSources embed.FS

func newBuiltinArtifact() (core.Artifact, error) {
	names := []string{"actions.go", "codec.go", "manifest.go", "results.go", "ruleset.go", "source.go", "state.go"}
	sources := make(map[string]string, len(names)+2)
	for _, name := range names {
		content, err := builtinSources.ReadFile(name)
		if err != nil {
			return core.Artifact{}, fmt.Errorf("dnd5e: read embedded source %s: %w", name, err)
		}
		sources["package/dnd5e/"+name] = string(content)
	}
	sources["shared/diceexpr"] = diceexpr.SourceIdentity()
	sources["shared/jsonstrict"] = jsonstrict.SourceIdentity()
	return core.NewBuiltinArtifact(packageManifest(), sources)
}
