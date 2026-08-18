package dnd5e

import (
	"strings"

	core "github.com/theburrowhub/thaimaturgy/internal/rules"
)

const (
	// PackageID is the stable built-in rules package identifier.
	PackageID = "dnd5e"
	// PackageVersion changes only when the built-in rules behavior changes.
	PackageVersion = "0.1.0"
)

// artifactMaterial is the stable byte identity of this built-in compatibility
// artifact. Changes to mechanics, schemas, or outcomes require a package version
// bump and a corresponding material update.
const artifactMaterial = "thaimaturgy builtin rules artifact\npackage=dnd5e\nversion=0.1.0\nprotocol=1.0.0\nactions=ability.check,dice.roll\nabi=1\n"

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
	return core.NewArtifact(packageManifest(), strings.NewReader(artifactMaterial))
}

// Ruleset implements the built-in D&D 5e compatibility package.
type Ruleset struct{}

// New returns a fresh stateless ruleset instance.
func New() *Ruleset { return &Ruleset{} }
