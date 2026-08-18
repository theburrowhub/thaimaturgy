// Package runtimecatalog assembles the rules packages available to a running
// thAImaturgy process. Built-ins are always registered first; independently
// installed Starlark bundles are then discovered from the application data
// directory without making adventure content executable.
package runtimecatalog

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/theburrowhub/thaimaturgy/internal/rules"
	"github.com/theburrowhub/thaimaturgy/internal/rules/bundlestore"
	"github.com/theburrowhub/thaimaturgy/internal/rules/catalog"
	"github.com/theburrowhub/thaimaturgy/internal/rules/coc7e"
	"github.com/theburrowhub/thaimaturgy/internal/rules/dnd5e"
	"github.com/theburrowhub/thaimaturgy/internal/rules/fatecore"
	"github.com/theburrowhub/thaimaturgy/internal/rules/gurps4e"
	"github.com/theburrowhub/thaimaturgy/internal/rules/pbta"
	"github.com/theburrowhub/thaimaturgy/internal/rules/pf2e"
	"github.com/theburrowhub/thaimaturgy/internal/rules/runequest"
	"github.com/theburrowhub/thaimaturgy/internal/rules/savageworlds"
	"github.com/theburrowhub/thaimaturgy/internal/rules/shadowrun6e"
	"github.com/theburrowhub/thaimaturgy/internal/rules/vtm5e"
)

// Environment is one immutable-at-startup view of the executable rules
// packages available to a process. Diagnostics contains non-fatal failures for
// individual external bundles: healthy packages remain registered, while an
// exact session lock for a rejected bundle still fails closed on lookup.
type Environment struct {
	Catalog       *catalog.Catalog
	Store         *bundlestore.Store
	DataDirectory string
	ExternalLocks []rules.Lock
	Diagnostics   error
}

// builtinDefinition is one immutable built-in release implementation. Old
// definitions stay in this list for as long as their entry remains in the
// append-only release ledger, so persisted exact locks remain loadable.
type builtinDefinition struct {
	id       string
	artifact func() (rules.Artifact, error)
	ruleset  func() rules.Ruleset
	initial  func() rules.Payload
}

// builtinDefinitions returns every built-in release implemented by this host.
// Adding mechanics requires a new package version and a new ledger entry;
// released definitions must not be edited or removed.
func builtinDefinitions() []builtinDefinition {
	return []builtinDefinition{
		{dnd5e.PackageID, dnd5e.NewArtifact, func() rules.Ruleset { return dnd5e.New() }, dnd5e.InitialState},
		{pf2e.PackageID, pf2e.NewArtifact, func() rules.Ruleset { return pf2e.New() }, pf2e.InitialState},
		{runequest.PackageID, runequest.NewArtifact, func() rules.Ruleset { return runequest.New() }, runequest.InitialState},
		{coc7e.PackageID, coc7e.NewArtifact, func() rules.Ruleset { return coc7e.New() }, coc7e.InitialState},
		{vtm5e.PackageID, vtm5e.NewArtifact, func() rules.Ruleset { return vtm5e.New() }, vtm5e.InitialState},
		{shadowrun6e.PackageID, shadowrun6e.NewArtifact, func() rules.Ruleset { return shadowrun6e.New() }, shadowrun6e.InitialState},
		{pbta.PackageID, pbta.NewArtifact, func() rules.Ruleset { return pbta.New() }, pbta.InitialState},
		{gurps4e.PackageID, gurps4e.NewArtifact, func() rules.Ruleset { return gurps4e.New() }, gurps4e.InitialState},
		{fatecore.PackageID, fatecore.NewArtifact, func() rules.Ruleset { return fatecore.New() }, fatecore.InitialState},
		{savageworlds.PackageID, savageworlds.NewArtifact, func() rules.Ruleset { return savageworlds.New() }, savageworlds.InitialState},
	}
}

// Load creates a process-local catalog from trusted built-ins and the dedicated
// external rules store below dataDirectory. A failure to create the catalog or
// store is fatal; malformed individual bundles are reported in Diagnostics.
func Load(ctx context.Context, dataDirectory string) (*Environment, error) {
	if ctx == nil {
		return nil, errors.New("rules runtime catalog: nil context")
	}
	if strings.TrimSpace(dataDirectory) == "" {
		return nil, errors.New("rules runtime catalog: data directory is required")
	}
	dataDirectory = strings.TrimSpace(dataDirectory)

	dataDirectory, err := filepath.Abs(dataDirectory)
	if err != nil {
		return nil, fmt.Errorf("rules runtime catalog: resolve data directory: %w", err)
	}
	available := catalog.New()
	if err := registerBuiltins(ctx, available); err != nil {
		return nil, fmt.Errorf("rules runtime catalog: register built-ins: %w", err)
	}
	store, err := bundlestore.New(filepath.Join(dataDirectory, bundlestore.DirectoryName), nil)
	if err != nil {
		return nil, fmt.Errorf("rules runtime catalog: open external store: %w", err)
	}
	external, diagnostics := store.RegisterAll(ctx, available)
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	return &Environment{
		Catalog:       available,
		Store:         store,
		DataDirectory: dataDirectory,
		ExternalLocks: append([]rules.Lock(nil), external...),
		Diagnostics:   diagnostics,
	}, nil
}

func registerBuiltins(ctx context.Context, destination *catalog.Catalog) error {
	prepared, err := prepareBuiltins()
	if err != nil {
		return err
	}
	for _, candidate := range prepared {
		if err := destination.Register(ctx, candidate.artifact, candidate.definition.ruleset(), candidate.definition.initial()); err != nil {
			return fmt.Errorf("register %s: %w", candidate.definition.id, err)
		}
	}
	return nil
}
