package rulesystem

import (
	"fmt"
	"strings"
)

// InspectReport returns a rich text summary of a pack's contents.
func InspectReport(p *Pack) string {
	if p == nil {
		return "pack: <nil>"
	}
	var b strings.Builder
	fmt.Fprintf(&b, "Pack: %s (%s)\n", p.Name, p.ID)
	fmt.Fprintf(&b, "API: %s  Version: %s  Family: %s\n\n", p.APIVersion, p.Version, p.Family)

	fmt.Fprintf(&b, "Core counts:\n")
	fmt.Fprintf(&b, "  Attributes: %d  Skills: %d  Resources: %d  Conditions: %d\n",
		len(p.Attributes), len(p.Skills), len(p.Resources), len(p.Conditions))
	fmt.Fprintf(&b, "  Workflows: %d  Mechanics: %d  Tables: %d  Chapters: %d  Tools: %d\n\n",
		len(p.Workflows), len(p.Mechanics), len(p.Tables), len(p.Chapters), len(p.Tools))

	if p.Dice.Primary != "" {
		fmt.Fprintf(&b, "Dice: primary=%s notation=%s\n", p.Dice.Primary, p.Dice.Notation)
	}
	if p.Combat.Mode != "" {
		fmt.Fprintf(&b, "Combat: mode=%s round_steps=%d actions=%d\n",
			p.Combat.Mode, len(p.Combat.RoundSteps), len(p.Combat.ActionEconomy.Actions))
	}
	if p.Progression.Kind != "" {
		fmt.Fprintf(&b, "Progression: kind=%s levels=%d\n", p.Progression.Kind, len(p.Progression.Levels))
	}
	fmt.Fprintln(&b)

	if len(p.Attributes) > 0 {
		fmt.Fprintln(&b, "Attributes:")
		for _, a := range p.Attributes {
			fmt.Fprintf(&b, "  - %s (%s)\n", a.Label, a.ID)
		}
		fmt.Fprintln(&b)
	}
	if len(p.Skills) > 0 {
		fmt.Fprintln(&b, "Skills:")
		for _, s := range p.Skills {
			fmt.Fprintf(&b, "  - %s -> %s\n", s.Label, s.Attribute)
		}
		fmt.Fprintln(&b)
	}
	if len(p.Workflows) > 0 {
		fmt.Fprintln(&b, "Workflows:")
		for _, w := range p.Workflows {
			fmt.Fprintf(&b, "  - %s [%s] steps=%d\n", w.Label, w.Category, len(w.Steps))
		}
		fmt.Fprintln(&b)
	}
	if len(p.Chapters) > 0 {
		fmt.Fprintln(&b, "Chapters:")
		for _, ch := range p.Chapters {
			fmt.Fprintf(&b, "  - %s: %s (%d sections)\n", ch.ID, ch.Title, len(ch.Sections))
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
	if len(p.RulesSummary) > 0 {
		fmt.Fprintln(&b)
		fmt.Fprintln(&b, "Rules summary:")
		for _, line := range p.RulesSummary {
			fmt.Fprintf(&b, "  • %s\n", line)
		}
	}
	return strings.TrimRight(b.String(), "\n")
}
