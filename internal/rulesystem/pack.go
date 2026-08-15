package rulesystem

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

// LoadPack reads a pack from JSON or YAML based on file extension.
func LoadPack(path string) (*Pack, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var p Pack
	ext := strings.ToLower(filepath.Ext(path))
	switch ext {
	case ".yaml", ".yml":
		if err := yaml.Unmarshal(data, &p); err != nil {
			return nil, err
		}
	default:
		if err := json.Unmarshal(data, &p); err != nil {
			return nil, err
		}
	}
	if err := ValidatePackStrict(&p); err != nil {
		return nil, fmt.Errorf("invalid pack: %w", err)
	}
	return &p, nil
}

// Load is an alias for LoadPack.
func Load(path string) (*Pack, error) { return LoadPack(path) }

// Save is an alias for SavePack.
func Save(p *Pack, path string) error { return SavePack(path, p) }

// SavePack writes a pack as JSON or YAML based on file extension.
func SavePack(path string, p *Pack) error {
	if p == nil {
		return fmt.Errorf("nil pack")
	}
	if p.APIVersion == "" {
		p.APIVersion = APIVersion
	}
	if err := ValidatePackStrict(p); err != nil {
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
	return os.WriteFile(path, data, 0o644)
}

// DiffSummary compares two packs and returns a human-readable summary of differences.
func DiffSummary(a, b *Pack) string {
	if a == nil && b == nil {
		return "both packs are nil"
	}
	if a == nil {
		return fmt.Sprintf("added pack %q", b.ID)
	}
	if b == nil {
		return fmt.Sprintf("removed pack %q", a.ID)
	}
	var lines []string
	if a.ID != b.ID {
		lines = append(lines, fmt.Sprintf("id: %q -> %q", a.ID, b.ID))
	}
	if a.Name != b.Name {
		lines = append(lines, fmt.Sprintf("name: %q -> %q", a.Name, b.Name))
	}
	if a.Version != b.Version {
		lines = append(lines, fmt.Sprintf("version: %q -> %q", a.Version, b.Version))
	}
	lines = append(lines, countDiff("attributes", len(a.Attributes), len(b.Attributes))...)
	lines = append(lines, countDiff("skills", len(a.Skills), len(b.Skills))...)
	lines = append(lines, countDiff("conditions", len(a.Conditions), len(b.Conditions))...)
	lines = append(lines, countDiff("workflows", len(a.Workflows), len(b.Workflows))...)
	lines = append(lines, countDiff("mechanics", len(a.Mechanics), len(b.Mechanics))...)
	lines = append(lines, countDiff("tables", len(a.Tables), len(b.Tables))...)
	lines = append(lines, countDiff("chapters", len(a.Chapters), len(b.Chapters))...)
	lines = append(lines, countDiff("tools", len(a.Tools), len(b.Tools))...)
	lines = append(lines, countDiff("formulas", len(a.Formulas), len(b.Formulas))...)
	if len(lines) == 0 {
		return "no structural differences detected"
	}
	return strings.Join(lines, "\n")
}

func countDiff(label string, oldN, newN int) []string {
	if oldN == newN {
		return nil
	}
	return []string{fmt.Sprintf("%s: %d -> %d (%+d)", label, oldN, newN, newN-oldN)}
}
