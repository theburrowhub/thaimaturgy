package storage

import (
	"os"
	"path/filepath"
	"testing"
)

func TestPackageAndImportRoundTrip(t *testing.T) {
	// Build a module directory as the editor would.
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, AdventureFile), []byte(validAdventureJSON), 0644); err != nil {
		t.Fatal(err)
	}
	for _, rel := range []string{"assets/map.png", "assets/guard.png"} {
		p := filepath.Join(dir, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(p), 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte("PNG"), 0644); err != nil {
			t.Fatal(err)
		}
	}
	// A hidden file must be excluded from the package.
	if err := os.WriteFile(filepath.Join(dir, ".DS_Store"), []byte("junk"), 0644); err != nil {
		t.Fatal(err)
	}

	dest := filepath.Join(t.TempDir(), "out.tar.gz")
	if err := PackageModule(dir, dest); err != nil {
		t.Fatalf("PackageModule: %v", err)
	}

	// The packaged archive must import and validate cleanly.
	store, _ := NewWithPath(t.TempDir())
	adv, err := store.ImportModule(dest)
	if err != nil {
		t.Fatalf("ImportModule of packaged module: %v", err)
	}
	if adv.ID != "crypt" {
		t.Errorf("ID = %q, want crypt", adv.ID)
	}
}

func TestPackageModuleMissingAdventure(t *testing.T) {
	dir := t.TempDir()
	dest := filepath.Join(t.TempDir(), "out.tar.gz")
	if err := PackageModule(dir, dest); err == nil {
		t.Fatal("expected error when adventure.json is missing")
	}
}

func TestExtractModuleRoundTrip(t *testing.T) {
	src := buildTarGz(t, map[string][]byte{
		"adventure.json":   []byte(validAdventureJSON),
		"assets/map.png":   []byte("PNG"),
		"assets/guard.png": []byte("PNG"),
	})
	dir := t.TempDir()
	if err := ExtractModule(src, dir); err != nil {
		t.Fatalf("ExtractModule: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dir, AdventureFile)); err != nil {
		t.Errorf("adventure.json not extracted: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dir, "assets", "map.png")); err != nil {
		t.Errorf("asset not extracted: %v", err)
	}
}
