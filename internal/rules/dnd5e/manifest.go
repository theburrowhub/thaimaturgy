package dnd5e

import core "github.com/theburrowhub/thaimaturgy/internal/rules"

const (
	// PackageID is the stable built-in rules package identifier.
	PackageID = "dnd5e"
	// PackageVersion changes only when the built-in rules behavior changes.
	PackageVersion = "0.1.0"
)

func packageManifest() core.Manifest {
	return core.Manifest{
		ID:              PackageID,
		Name:            "D&D 5e compatibility rules",
		Description:     "Built-in compatibility rules for the existing dice roll and ability check tools.",
		Version:         PackageVersion,
		ProtocolVersion: core.ProtocolVersion,
		Runtime:         core.Runtime{Kind: core.RuntimeBuiltin},
		Capabilities:    []string{ActionAbilityCheck, ActionDiceRoll},
	}
}

// NewArtifact returns the host-verifiable, stable artifact record for the
// built-in ruleset.
func NewArtifact() (core.Artifact, error) {
	return newBuiltinArtifact()
}

// Ruleset implements the built-in D&D 5e compatibility package.
type Ruleset struct{}

// New returns a fresh stateless ruleset instance.
func New() *Ruleset { return &Ruleset{} }
