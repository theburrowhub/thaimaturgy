package domain

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

// TestExampleAdventuresValidate loads every bundled example module and runs it
// through the same parse + Migrate + ValidateAdventure path an import uses, so a
// shipped example can never be structurally broken (referenced ids, images,
// scenes, etc.).
func TestExampleAdventuresValidate(t *testing.T) {
	root := filepath.Join("..", "..", "examples", "adventures")
	entries, err := os.ReadDir(root)
	if err != nil {
		t.Fatalf("read examples dir: %v", err)
	}
	found := 0
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		dir := filepath.Join(root, e.Name())
		data, err := os.ReadFile(filepath.Join(dir, "adventure.json"))
		if err != nil {
			continue // a dir without an adventure.json isn't a module
		}
		found++
		t.Run(e.Name(), func(t *testing.T) {
			var a Adventure
			if err := json.Unmarshal(data, &a); err != nil {
				t.Fatalf("parse: %v", err)
			}
			a.Migrate()
			imageExists := func(rel string) bool {
				_, err := os.Stat(filepath.Join(dir, rel))
				return err == nil
			}
			for _, verr := range ValidateAdventure(&a, imageExists) {
				t.Errorf("validation: %v", verr)
			}
		})
	}
	if found == 0 {
		t.Fatal("no example adventures found to validate")
	}
}
