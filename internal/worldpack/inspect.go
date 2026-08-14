package worldpack

import (
	"fmt"
	"strings"
)

// InspectReport returns a rich text summary of a world pack's contents.
func InspectReport(p *Pack) string {
	if p == nil {
		return "pack: <nil>"
	}
	var b strings.Builder
	fmt.Fprintf(&b, "World Pack: %s (%s)\n", p.Name, p.ID)
	fmt.Fprintf(&b, "API: %s  Version: %s  Setting: %s\n\n", p.APIVersion, p.Version, p.Setting.Name)

	fmt.Fprintf(&b, "Core counts:\n")
	fmt.Fprintf(&b, "  Regions: %d  Cities: %d  Locations: %d\n", len(p.Regions), len(p.Cities), len(p.Locations))
	fmt.Fprintf(&b, "  NPCs: %d  Creatures: %d  Items: %d  Factions: %d\n", len(p.NPCs), len(p.Creatures), len(p.Items), len(p.Factions))
	fmt.Fprintf(&b, "  Encounter tables: %d  Lore: %d  Maps: %d  Tools: %d\n\n",
		len(p.EncounterTables), len(p.Lore), len(p.Maps), len(p.Tools))

	fmt.Fprintf(&b, "Magic: %s\n", p.Setting.WorldRules.Magic)
	if p.Setting.SuggestedRulesystem != "" {
		fmt.Fprintf(&b, "Suggested rules: %s  Playable with: %v\n", p.Setting.SuggestedRulesystem, p.Setting.PlayableWith)
	}
	fmt.Fprintf(&b, "Politics: %s\n", p.Setting.Politics.Summary)
	fmt.Fprintf(&b, "Tone: %s  Era: %s\n\n", p.Setting.Tone, p.Setting.Era)


	if len(p.Regions) > 0 {
		fmt.Fprintln(&b, "Regions:")
		for _, r := range p.Regions {
			fmt.Fprintf(&b, "  - %s (%s) biome=%s cities=%d locs=%d\n", r.Name, r.ID, r.Biome, len(r.CityIDs), len(r.LocationIDs))
		}
		fmt.Fprintln(&b)
	}
	if len(p.Cities) > 0 {
		fmt.Fprintln(&b, "Cities:")
		for _, c := range p.Cities {
			fmt.Fprintf(&b, "  - %s (%s) districts=%d locations=%d\n", c.Name, c.ID, len(c.Districts), len(c.LocationIDs))
		}
		fmt.Fprintln(&b)
	}
	if len(p.Creatures) > 0 {
		fmt.Fprintln(&b, "Bestiary sample:")
		limit := len(p.Creatures)
		if limit > 5 {
			limit = 5
		}
		for _, c := range p.Creatures[:limit] {
			fmt.Fprintf(&b, "  - %s CR=%s habitats=%v\n", c.Name, c.CR, c.Habitats)
		}
		if len(p.Creatures) > 5 {
			fmt.Fprintf(&b, "  ... and %d more\n", len(p.Creatures)-5)
		}
		fmt.Fprintln(&b)
	}
	if len(p.EncounterTables) > 0 {
		fmt.Fprintln(&b, "Encounter tables:")
		for _, t := range p.EncounterTables {
			fmt.Fprintf(&b, "  - %s [%s] rows=%d dice=%s\n", t.Name, t.Context, len(t.Rows), t.Dice)
		}
		fmt.Fprintln(&b)
	}
	if len(p.Tools) > 0 {
		fmt.Fprintln(&b, "Tool bindings:")
		for _, t := range p.Tools {
			status := "enabled"
			if !t.Enabled {
				status = "disabled"
			}
			fmt.Fprintf(&b, "  - %s -> %s (%s)\n", t.Name, t.CanonicalID, status)
		}
		fmt.Fprintln(&b)
	}
	if len(p.OracleGuide.Scenarios) > 0 {
		fmt.Fprintln(&b, "Oracle scenarios:")
		for _, sc := range p.OracleGuide.Scenarios {
			fmt.Fprintf(&b, "  - %s tools=%v\n", sc.Situation, sc.UseTools)
		}
	}
	if p.Indexes.ByCity != nil {
		fmt.Fprintf(&b, "\nIndex: %d cities, %d regions, %d habitats, %d tags\n",
			len(p.Indexes.ByCity), len(p.Indexes.ByRegion), len(p.Indexes.ByCreatureHabitat), len(p.Indexes.ByTag))
	}
	return strings.TrimRight(b.String(), "\n")
}
