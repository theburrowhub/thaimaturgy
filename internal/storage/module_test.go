package storage

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"os"
	"path/filepath"
	"testing"
)

// buildTarGz writes a gzip-compressed tar archive of the given files to a temp
// file and returns its path. Keys are archive-relative paths.
func buildTarGz(t *testing.T, files map[string][]byte) string {
	t.Helper()
	var buf bytes.Buffer
	gz := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gz)
	for name, data := range files {
		hdr := &tar.Header{Name: name, Mode: 0644, Size: int64(len(data)), Typeflag: tar.TypeReg}
		if err := tw.WriteHeader(hdr); err != nil {
			t.Fatalf("WriteHeader: %v", err)
		}
		if _, err := tw.Write(data); err != nil {
			t.Fatalf("Write: %v", err)
		}
	}
	if err := tw.Close(); err != nil {
		t.Fatalf("tar Close: %v", err)
	}
	if err := gz.Close(); err != nil {
		t.Fatalf("gzip Close: %v", err)
	}

	dir := t.TempDir()
	path := filepath.Join(dir, "module.tar.gz")
	if err := os.WriteFile(path, buf.Bytes(), 0644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	return path
}

const validAdventureJSON = `{
  "schema_version": "1.0",
  "id": "crypt",
  "title": "The Crypt",
  "zones": [
    {"id":"z1","name":"Entrance","map_image":"assets/map.png",
     "rooms":[{"id":"r1","name":"Gate","npc_ids":["guard"]}]}
  ],
  "npcs": [{"id":"guard","name":"Gate Guard","image":"assets/guard.png"}]
}`

func TestImportModuleValid(t *testing.T) {
	src := buildTarGz(t, map[string][]byte{
		"adventure.json":   []byte(validAdventureJSON),
		"assets/map.png":   []byte("PNG"),
		"assets/guard.png": []byte("PNG"),
	})

	store, _ := NewWithPath(t.TempDir())
	adv, err := store.ImportModule(src)
	if err != nil {
		t.Fatalf("ImportModule failed: %v", err)
	}
	if adv.ID != "crypt" {
		t.Errorf("ID = %q, want crypt", adv.ID)
	}
	if !store.AdventureExists("crypt") {
		t.Error("AdventureExists should be true after import")
	}
	if _, err := store.ResolveImagePath("crypt", "assets/map.png"); err != nil {
		t.Errorf("ResolveImagePath failed: %v", err)
	}
	list, _ := store.ListAdventures()
	if len(list) != 1 {
		t.Errorf("ListAdventures len = %d, want 1", len(list))
	}
}

func TestImportModuleZipSlip(t *testing.T) {
	src := buildTarGz(t, map[string][]byte{
		"adventure.json":   []byte(validAdventureJSON),
		"assets/map.png":   []byte("PNG"),
		"assets/guard.png": []byte("PNG"),
		"../evil.txt":      []byte("owned"),
	})

	base := t.TempDir()
	store, _ := NewWithPath(base)
	if _, err := store.ImportModule(src); err == nil {
		t.Fatal("expected ImportModule to reject a path-traversal entry")
	}
	// The traversal target must not have been written outside the base dir.
	if _, err := os.Stat(filepath.Join(filepath.Dir(base), "evil.txt")); err == nil {
		t.Fatal("zip-slip wrote a file outside the destination")
	}
}

func TestImportModuleMissingImage(t *testing.T) {
	src := buildTarGz(t, map[string][]byte{
		"adventure.json": []byte(validAdventureJSON),
		"assets/map.png": []byte("PNG"),
		// assets/guard.png intentionally missing
	})
	store, _ := NewWithPath(t.TempDir())
	if _, err := store.ImportModule(src); err == nil {
		t.Fatal("expected validation error for missing image asset")
	}
}

func TestImportModuleBadReference(t *testing.T) {
	badJSON := `{
	  "schema_version":"1.0","id":"bad","title":"Bad",
	  "zones":[{"id":"z1","name":"Z","rooms":[{"id":"r1","name":"R","npc_ids":["ghost"]}]}]
	}`
	src := buildTarGz(t, map[string][]byte{"adventure.json": []byte(badJSON)})
	store, _ := NewWithPath(t.TempDir())
	if _, err := store.ImportModule(src); err == nil {
		t.Fatal("expected validation error for unknown npc reference")
	}
}

func TestImportModuleMissingAdventureJSON(t *testing.T) {
	src := buildTarGz(t, map[string][]byte{"assets/map.png": []byte("PNG")})
	store, _ := NewWithPath(t.TempDir())
	if _, err := store.ImportModule(src); err == nil {
		t.Fatal("expected error when adventure.json is absent")
	}
}

// dirToTarGz packages a directory's contents (relative paths) into a .tar.gz.
func dirToTarGz(t *testing.T, dir string) string {
	t.Helper()
	files := map[string][]byte{}
	err := filepath.Walk(dir, func(p string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			return nil
		}
		rel, err := filepath.Rel(dir, p)
		if err != nil {
			return err
		}
		data, err := os.ReadFile(p)
		if err != nil {
			return err
		}
		files[filepath.ToSlash(rel)] = data
		return nil
	})
	if err != nil {
		t.Fatalf("walk: %v", err)
	}
	return buildTarGz(t, files)
}

// TestImportExampleModule guards the shipped example adventure: it must package,
// import, and validate cleanly.
func TestImportExampleModule(t *testing.T) {
	exampleDir := filepath.Join("..", "..", "examples", "adventures", "the-sunken-crypt")
	if _, err := os.Stat(filepath.Join(exampleDir, "adventure.json")); err != nil {
		t.Skipf("example adventure not present: %v", err)
	}
	src := dirToTarGz(t, exampleDir)
	store, _ := NewWithPath(t.TempDir())
	adv, err := store.ImportModule(src)
	if err != nil {
		t.Fatalf("example module failed to import: %v", err)
	}
	if adv.ID != "the-sunken-crypt" {
		t.Errorf("ID = %q, want the-sunken-crypt", adv.ID)
	}
	if len(adv.Zones) < 2 || len(adv.NPCs) < 3 {
		t.Errorf("example seems thin: %d zones, %d npcs", len(adv.Zones), len(adv.NPCs))
	}
	for _, ref := range adv.ImageRefs() {
		if _, err := store.ResolveImagePath(adv.ID, ref); err != nil {
			t.Errorf("image %q not resolvable: %v", ref, err)
		}
	}
}
