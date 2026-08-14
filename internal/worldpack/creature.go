package worldpack

import (
	"github.com/theburrowhub/thaimaturgy/internal/domain"
)

// StatBlockFor returns the stat block for the given rulesystem ID, or default StatBlock.
func (c CreatureEntry) StatBlockFor(rulesystemID string) domain.StatBlock {
	if rulesystemID != "" && c.StatBlocks != nil {
		if sb, ok := c.StatBlocks[rulesystemID]; ok {
			return sb
		}
	}
	return c.StatBlock
}

// NormalizeCreatureEntry fills StatBlock from StatBlocks when only variants exist.
func NormalizeCreatureEntry(c *CreatureEntry) {
	if c == nil || len(c.StatBlocks) == 0 {
		return
	}
	empty := c.StatBlock.Type == "" && c.StatBlock.CR == "" && len(c.StatBlock.Traits) == 0
	if !empty {
		if c.CR == "" && c.StatBlock.CR != "" {
			c.CR = c.StatBlock.CR
		}
		return
	}
	for _, key := range []string{"dnd5e", "d100", "savage_worlds"} {
		if sb, ok := c.StatBlocks[key]; ok {
			c.StatBlock = sb
			break
		}
	}
	if c.StatBlock.Type == "" && c.StatBlock.CR == "" {
		for _, sb := range c.StatBlocks {
			c.StatBlock = sb
			break
		}
	}
	if c.CR == "" && c.StatBlock.CR != "" {
		c.CR = c.StatBlock.CR
	}
}

// AddCreature appends a bestiary entry with normalization.
func AddCreature(p *Pack, entry CreatureEntry) {
	NormalizeCreatureEntry(&entry)
	p.Creatures = append(p.Creatures, entry)
}

// AddCreatureMulti adds a creature with per-rulesystem stat blocks.
func AddCreatureMulti(p *Pack, id, name string, statBlocks map[string]domain.StatBlock, habitats []string, encounterNotes, lore string, tags ...string) {
	entry := CreatureEntry{
		ID: id, Name: name, StatBlocks: statBlocks,
		Habitats: habitats, EncounterNotes: encounterNotes, Lore: lore, Tags: tags,
		ToolAdapter: "lookup_creature",
	}
	NormalizeCreatureEntry(&entry)
	p.Creatures = append(p.Creatures, entry)
}
