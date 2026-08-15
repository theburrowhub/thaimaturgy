package worldpack

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/theburrowhub/thaimaturgy/internal/domain"
	"gopkg.in/yaml.v3"
)

// LoadPack reads a world pack from JSON or YAML based on file extension.
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
	BuildIndexes(&p)
	if err := ValidatePackStrict(&p); err != nil {
		return nil, fmt.Errorf("invalid pack: %w", err)
	}
	return &p, nil
}

// Load is an alias for LoadPack.
func Load(path string) (*Pack, error) { return LoadPack(path) }

// SavePack writes a world pack as JSON or YAML based on file extension.
func SavePack(path string, p *Pack) error {
	if p == nil {
		return fmt.Errorf("nil pack")
	}
	if p.APIVersion == "" {
		p.APIVersion = APIVersion
	}
	BuildIndexes(p)
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

// Save is an alias for SavePack.
func Save(p *Pack, path string) error { return SavePack(path, p) }

// DiffSummary compares two packs and returns a human-readable summary.
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
	lines = append(lines, countDiff("regions", len(a.Regions), len(b.Regions))...)
	lines = append(lines, countDiff("cities", len(a.Cities), len(b.Cities))...)
	lines = append(lines, countDiff("locations", len(a.Locations), len(b.Locations))...)
	lines = append(lines, countDiff("npcs", len(a.NPCs), len(b.NPCs))...)
	lines = append(lines, countDiff("creatures", len(a.Creatures), len(b.Creatures))...)
	lines = append(lines, countDiff("items", len(a.Items), len(b.Items))...)
	lines = append(lines, countDiff("encounter_tables", len(a.EncounterTables), len(b.EncounterTables))...)
	lines = append(lines, countDiff("tools", len(a.Tools), len(b.Tools))...)
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

// Pack returns entity lookups on a pack.

func (p *Pack) Region(id string) *Region {
	for i := range p.Regions {
		if p.Regions[i].ID == id {
			return &p.Regions[i]
		}
	}
	return nil
}

func (p *Pack) City(id string) *City {
	for i := range p.Cities {
		if p.Cities[i].ID == id {
			return &p.Cities[i]
		}
	}
	return nil
}

func (p *Pack) District(cityID, districtID string) *District {
	c := p.City(cityID)
	if c == nil {
		return nil
	}
	for i := range c.Districts {
		if c.Districts[i].ID == districtID {
			return &c.Districts[i]
		}
	}
	return nil
}

func (p *Pack) Location(id string) *Location {
	for i := range p.Locations {
		if p.Locations[i].ID == id {
			return &p.Locations[i]
		}
	}
	return nil
}

func (p *Pack) NPC(id string) *WorldNPC {
	for i := range p.NPCs {
		if p.NPCs[i].ID == id {
			return &p.NPCs[i]
		}
	}
	return nil
}

func (p *Pack) Creature(id string) *CreatureEntry {
	for i := range p.Creatures {
		if p.Creatures[i].ID == id {
			return &p.Creatures[i]
		}
	}
	return nil
}

func (p *Pack) Item(id string) *WorldItem {
	for i := range p.Items {
		if p.Items[i].ID == id {
			return &p.Items[i]
		}
	}
	return nil
}

func (p *Pack) Faction(id string) *domain.Faction {
	for i := range p.Factions {
		if p.Factions[i].ID == id {
			return &p.Factions[i]
		}
	}
	return nil
}

func (p *Pack) LoreEntry(id string) *LoreEntry {
	for i := range p.Lore {
		if p.Lore[i].ID == id {
			return &p.Lore[i]
		}
	}
	return nil
}

func (p *Pack) Map(id string) *MapRef {
	for i := range p.Maps {
		if p.Maps[i].ID == id {
			return &p.Maps[i]
		}
	}
	return nil
}

func (p *Pack) EncounterTable(id string) *EncounterTable {
	for i := range p.EncounterTables {
		if p.EncounterTables[i].ID == id {
			return &p.EncounterTables[i]
		}
	}
	return nil
}

func (p *Pack) LocationContentsFor(locationID string) *LocationContents {
	for i := range p.LocationContents {
		if p.LocationContents[i].LocationID == locationID {
			return &p.LocationContents[i]
		}
	}
	return nil
}
