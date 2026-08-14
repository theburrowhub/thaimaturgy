package rulesystem

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

// Load reads a pack from JSON or YAML (.json, .yaml, .yml).
func Load(path string) (*Pack, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var p Pack
	ext := strings.ToLower(filepath.Ext(path))
	switch ext {
	case ".yaml", ".yml":
		if err := yaml.Unmarshal(data, &p); err != nil {
			return nil, fmt.Errorf("parse yaml: %w", err)
		}
	default:
		if err := json.Unmarshal(data, &p); err != nil {
			return nil, fmt.Errorf("parse json: %w", err)
		}
	}
	if err := Validate(&p); err != nil {
		return nil, fmt.Errorf("invalid pack: %w", err)
	}
	return &p, nil
}

// Save writes a pack as indented JSON or YAML based on the output extension.
func Save(p *Pack, path string) error {
	if err := Validate(p); err != nil {
		return err
	}
	var data []byte
	var err error
	ext := strings.ToLower(filepath.Ext(path))
	switch ext {
	case ".yaml", ".yml":
		data, err = yaml.Marshal(p)
	default:
		data, err = json.MarshalIndent(p, "", "  ")
	}
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	return os.WriteFile(path, append(data, '\n'), 0o644)
}

// Format returns "json" or "yaml" for a path extension.
func Format(path string) string {
	switch strings.ToLower(filepath.Ext(path)) {
	case ".yaml", ".yml":
		return "yaml"
	default:
		return "json"
	}
}
