package runtimecatalog

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/theburrowhub/thaimaturgy/internal/rules/dnd5e"
)

func TestLoadRegistersBuiltinsAndCreatesSeparatedStore(t *testing.T) {
	dataDirectory := t.TempDir()
	environment, err := Load(context.Background(), dataDirectory)
	if err != nil {
		t.Fatal(err)
	}
	if environment.Diagnostics != nil {
		t.Fatalf("unexpected diagnostics: %v", environment.Diagnostics)
	}
	artifact, err := dnd5e.NewArtifact()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := environment.Catalog.Lookup(artifact.Lock()); err != nil {
		t.Fatalf("built-in dnd5e is unavailable: %v", err)
	}
	wantRoot := filepath.Join(dataDirectory, "rulesets")
	if environment.Store.Root() != wantRoot {
		t.Fatalf("store root = %q, want %q", environment.Store.Root(), wantRoot)
	}
	if info, err := os.Stat(wantRoot); err != nil || !info.IsDir() {
		t.Fatalf("external rules directory is unavailable: %v", err)
	}
}

func TestLoadKeepsHealthyCatalogWhenExternalBundleIsInvalid(t *testing.T) {
	dataDirectory := t.TempDir()
	badPath := filepath.Join(dataDirectory, "rulesets", "bad", "0.1.0")
	if err := os.MkdirAll(badPath, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(badPath, "invalid.rules.zip"), []byte("not a zip"), 0o600); err != nil {
		t.Fatal(err)
	}

	environment, err := Load(context.Background(), dataDirectory)
	if err != nil {
		t.Fatal(err)
	}
	if environment.Diagnostics == nil {
		t.Fatal("invalid external bundle did not produce diagnostics")
	}
	artifact, err := dnd5e.NewArtifact()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := environment.Catalog.Lookup(artifact.Lock()); err != nil {
		t.Fatalf("invalid external bundle hid healthy built-in: %v", err)
	}
}

func TestLoadRejectsMissingInputs(t *testing.T) {
	if _, err := Load(nil, t.TempDir()); err == nil {
		t.Fatal("nil context was accepted")
	}
	if _, err := Load(context.Background(), " "); err == nil {
		t.Fatal("empty data directory was accepted")
	}
}
