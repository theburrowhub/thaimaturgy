// Package dmbook renders an authored adventure module into a complete DM
// sourcebook (Markdown), faithful to the module's content — no AI involved. Pair
// it with internal/bookpdf to produce a print-ready PDF.
package dmbook

import (
	"fmt"
	"strings"

	"github.com/theburrowhub/thaimaturgy/internal/domain"
)

// Markdown renders the adventure as a DM sourcebook in GitHub-flavored Markdown:
// title page, overview/background/introduction/hooks/conclusion, then a chapter
// per zone (with its rooms), and chapters for NPCs, events and items.
func Markdown(adv *domain.Adventure) string {
	var b strings.Builder
	w := func(format string, args ...any) { fmt.Fprintf(&b, format, args...) }

	w("# %s\n\n", nz(adv.Title, "Untitled Adventure"))

	// --- Overview ---
	w("## Overview\n\n")
	if adv.System != "" {
		w("**System:** %s  \n", adv.System)
	}
	if adv.Author != "" {
		w("**Author:** %s  \n", adv.Author)
	}
	w("\n")
	writeField(&b, "", adv.Summary)
	writeSection(&b, "Background (DM only)", adv.Background)
	writeSection(&b, "Introduction", adv.Introduction)
	if len(adv.Hooks) > 0 {
		w("### Hooks\n\n")
		for _, h := range adv.Hooks {
			w("- %s\n", oneLine(h))
		}
		w("\n")
	}
	writeSection(&b, "Conclusion", adv.Conclusion)

	// --- Zones & rooms ---
	for i := range adv.Zones {
		z := &adv.Zones[i]
		w("## %s\n\n", nz(z.Name, z.ID))
		writeField(&b, "Overview", z.Overview)
		writeField(&b, "Description", z.Description)
		if len(z.Connections) > 0 {
			w("**Connects to:** %s\n\n", strings.Join(z.Connections, ", "))
		}
		if m := adv.ZoneMap(z); m != "" {
			w("*Map: %s*\n\n", m)
		}
		for ri := range z.Rooms {
			writeRoom(&b, adv, &z.Rooms[ri])
		}
	}

	// --- NPCs ---
	if len(adv.NPCs) > 0 {
		w("## Non-Player Characters\n\n")
		for i := range adv.NPCs {
			writeNPC(&b, &adv.NPCs[i])
		}
	}

	// --- Events ---
	if len(adv.Events) > 0 {
		w("## Events\n\n")
		for i := range adv.Events {
			writeEvent(&b, &adv.Events[i])
		}
	}

	// --- Items ---
	if len(adv.Items) > 0 {
		w("## Items\n\n")
		for i := range adv.Items {
			writeItem(&b, &adv.Items[i])
		}
	}

	return b.String()
}

func writeRoom(b *strings.Builder, adv *domain.Adventure, r *domain.Room) {
	fmt.Fprintf(b, "### %s\n\n", nz(r.Name, r.ID))
	if r.ReadAloud != "" {
		b.WriteString("**Read-aloud:**\n\n")
		for _, ln := range strings.Split(strings.TrimRight(r.ReadAloud, "\n"), "\n") {
			fmt.Fprintf(b, "> %s\n", ln)
		}
		b.WriteString("\n")
	}
	writeField(b, "DM notes", r.DMNotes)
	if len(r.NPCIDs) > 0 {
		var names []string
		for _, id := range r.NPCIDs {
			if n := adv.NPC(id); n != nil {
				names = append(names, n.Name)
			} else {
				names = append(names, id)
			}
		}
		fmt.Fprintf(b, "**NPCs present:** %s\n\n", strings.Join(names, ", "))
	}
	if len(r.Features) > 0 {
		b.WriteString("**Features:**\n\n")
		for _, f := range r.Features {
			line := f.Name
			if f.Skill != "" {
				line += fmt.Sprintf(" (%s", f.Skill)
				if f.DC > 0 {
					line += fmt.Sprintf(" DC %d", f.DC)
				}
				line += ")"
			}
			if f.Description != "" {
				line += ": " + f.Description
			}
			fmt.Fprintf(b, "- %s\n", oneLine(line))
			if f.Success != "" {
				fmt.Fprintf(b, "  - Success: %s\n", oneLine(f.Success))
			}
			if f.Failure != "" {
				fmt.Fprintf(b, "  - Failure: %s\n", oneLine(f.Failure))
			}
		}
		b.WriteString("\n")
	}
	if len(r.Encounters) > 0 {
		b.WriteString("**Encounters:**\n\n")
		for _, e := range r.Encounters {
			line := e.Name
			if e.Difficulty != "" {
				line += fmt.Sprintf(" [%s]", e.Difficulty)
			}
			if e.Description != "" {
				line += ": " + e.Description
			}
			fmt.Fprintf(b, "- %s\n", oneLine(line))
			if len(e.Creatures) > 0 {
				fmt.Fprintf(b, "  - Creatures: %s\n", strings.Join(e.Creatures, ", "))
			}
			if e.Tactics != "" {
				fmt.Fprintf(b, "  - Tactics: %s\n", oneLine(e.Tactics))
			}
		}
		b.WriteString("\n")
	}
	if len(r.Treasure) > 0 {
		fmt.Fprintf(b, "**Treasure:** %s\n\n", strings.Join(r.Treasure, ", "))
	}
	if len(r.Exits) > 0 {
		b.WriteString("**Exits:**\n\n")
		for _, ex := range r.Exits {
			label := ex.Direction
			if label == "" {
				label = "→"
			}
			line := fmt.Sprintf("%s to %s", label, ex.To)
			if ex.Locked {
				line += " (locked)"
			}
			if ex.Description != "" {
				line += ": " + ex.Description
			}
			fmt.Fprintf(b, "- %s\n", oneLine(line))
		}
		b.WriteString("\n")
	}
}

func writeNPC(b *strings.Builder, n *domain.NPC) {
	title := nz(n.Name, n.ID)
	if n.Role != "" {
		title += " — " + n.Role
	}
	fmt.Fprintf(b, "### %s\n\n", title)
	writeField(b, "Appearance", n.Appearance)
	writeField(b, "Personality", n.Personality)
	writeField(b, "Motivations", n.Motivations)
	writeField(b, "Secrets", n.Secrets)
	writeField(b, "Voice", n.Voice)
	writeField(b, "Disposition", n.Disposition)
	if n.DefaultLocation != "" {
		fmt.Fprintf(b, "**Location:** %s\n\n", n.DefaultLocation)
	}
	if len(n.Knowledge) > 0 {
		b.WriteString("**Knows:**\n\n")
		for _, k := range n.Knowledge {
			fmt.Fprintf(b, "- %s\n", oneLine(k))
		}
		b.WriteString("\n")
	}
	if len(n.SampleDialogue) > 0 {
		b.WriteString("**Sample dialogue:**\n\n")
		for _, d := range n.SampleDialogue {
			fmt.Fprintf(b, "> “%s”\n", oneLine(d))
		}
		b.WriteString("\n")
	}
	writeStatBlock(b, n.StatBlock)
}

func writeStatBlock(b *strings.Builder, s *domain.StatBlock) {
	if s == nil {
		return
	}
	b.WriteString("**Stat block:**\n\n")
	var parts []string
	if s.AC > 0 {
		parts = append(parts, fmt.Sprintf("AC %d", s.AC))
	}
	if s.MaxHP > 0 {
		parts = append(parts, fmt.Sprintf("HP %d", s.MaxHP))
	}
	if s.Speed != "" {
		parts = append(parts, "Speed "+s.Speed)
	}
	if s.CR != "" {
		parts = append(parts, "CR "+s.CR)
	}
	if len(parts) > 0 {
		fmt.Fprintf(b, "- %s\n", strings.Join(parts, " · "))
	}
	if a := s.Abilities; a != (domain.AbilityScores{}) {
		fmt.Fprintf(b, "- STR %d (%s) · DEX %d (%s) · CON %d (%s) · INT %d (%s) · WIS %d (%s) · CHA %d (%s)\n",
			a.STR, domain.ModifierString(a.STR), a.DEX, domain.ModifierString(a.DEX),
			a.CON, domain.ModifierString(a.CON), a.INT, domain.ModifierString(a.INT),
			a.WIS, domain.ModifierString(a.WIS), a.CHA, domain.ModifierString(a.CHA))
	}
	if len(s.Skills) > 0 {
		fmt.Fprintf(b, "- Skills: %s\n", strings.Join(s.Skills, ", "))
	}
	for _, t := range s.Traits {
		fmt.Fprintf(b, "- *Trait:* %s\n", oneLine(t))
	}
	for _, act := range s.Actions {
		line := act.Name
		if act.ToHit != "" {
			line += " (" + act.ToHit + " to hit"
			if act.Damage != "" {
				line += ", " + act.Damage
			}
			line += ")"
		} else if act.Damage != "" {
			line += " (" + act.Damage + ")"
		}
		if act.Description != "" {
			line += ": " + act.Description
		}
		fmt.Fprintf(b, "- *Action:* %s\n", oneLine(line))
	}
	b.WriteString("\n")
}

func writeEvent(b *strings.Builder, e *domain.Event) {
	fmt.Fprintf(b, "### %s\n\n", nz(e.Name, e.ID))
	writeField(b, "Trigger", e.Trigger)
	writeField(b, "Description", e.Description)
	if e.ReadAloud != "" {
		b.WriteString("**Read-aloud:**\n\n")
		for _, ln := range strings.Split(strings.TrimRight(e.ReadAloud, "\n"), "\n") {
			fmt.Fprintf(b, "> %s\n", ln)
		}
		b.WriteString("\n")
	}
	writeField(b, "DM notes", e.DMNotes)
	writeField(b, "Consequences", e.Consequences)
	if len(e.Outcomes) > 0 {
		b.WriteString("**Outcomes:**\n\n")
		for _, o := range e.Outcomes {
			fmt.Fprintf(b, "- If %s → %s\n", oneLine(o.Condition), oneLine(o.Result))
		}
		b.WriteString("\n")
	}
}

func writeItem(b *strings.Builder, it *domain.Item) {
	fmt.Fprintf(b, "### %s\n\n", nz(it.Name, it.ID))
	writeField(b, "Rarity", it.Rarity)
	writeField(b, "Description", it.Description)
	writeField(b, "Mechanics", it.Mechanics)
}

// writeField writes "**Label:** value" (or just the value when label is empty),
// followed by a blank line, skipping empty values.
func writeField(b *strings.Builder, label, value string) {
	value = strings.TrimSpace(value)
	if value == "" {
		return
	}
	if label == "" {
		fmt.Fprintf(b, "%s\n\n", value)
		return
	}
	fmt.Fprintf(b, "**%s:** %s\n\n", label, value)
}

// writeSection writes a "### Label" heading with a prose body, skipping if empty.
func writeSection(b *strings.Builder, label, value string) {
	if strings.TrimSpace(value) == "" {
		return
	}
	fmt.Fprintf(b, "### %s\n\n%s\n\n", label, strings.TrimSpace(value))
}

func oneLine(s string) string { return strings.Join(strings.Fields(s), " ") }

func nz(s, fallback string) string {
	if strings.TrimSpace(s) != "" {
		return s
	}
	return fallback
}
