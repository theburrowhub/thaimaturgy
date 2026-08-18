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
	"github.com/theburrowhub/thaimaturgy/internal/rules/dnd5e"
)

// Environment is one immutable-at-startup view of the executable rules
// packages available to a process. Diagnostics contains non-fatal failures for
// individual external bundles: healthy packages remain registered, while an
// exact session lock for a rejected bundle still fails closed on lookup.
type Environment struct {
	Catalog       *catalog.Catalog
	Store         *bundlestore.Store
	ExternalLocks []rules.Lock
	Diagnostics   error
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

	available := catalog.New()
	if err := registerBuiltins(ctx, available); err != nil {
		return nil, fmt.Errorf("rules runtime catalog: register built-ins: %w", err)
	}
	store, err := bundlestore.New(filepath.Join(dataDirectory, bundlestore.DirectoryName), nil)
	if err != nil {
		return nil, fmt.Errorf("rules runtime catalog: open external store: %w", err)
	}
	external, diagnostics := store.RegisterAll(ctx, available)
	return &Environment{
		Catalog:       available,
		Store:         store,
		ExternalLocks: append([]rules.Lock(nil), external...),
		Diagnostics:   diagnostics,
	}, nil
}

func registerBuiltins(ctx context.Context, destination *catalog.Catalog) error {
	implementation := dnd5e.New()
	artifact, err := dnd5e.NewArtifact()
	if err != nil {
		return err
	}
	return destination.Register(ctx, artifact, implementation, dnd5e.InitialState())
}
