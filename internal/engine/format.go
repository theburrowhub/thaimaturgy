package engine

import (
	"fmt"
	"strings"

	"github.com/theburrowhub/thaimaturgy/internal/domain"
)

// This file renders authored adventure content into readable text blocks,
// reused by the oracle context builder, the DM commands, and the TUI panels.

// FormatRoom renders a room with its read-aloud text and DM notes.
func FormatRoom(adv *domain.Adventure, r *domain.Room) string {
	if r == nil {
		return "(unknown room)"
	}
	var sb strings.Builder
	fmt.Fprintf(&sb, "ROOM: %s [%s]\n", r.Name, r.ID)
	if r.ReadAloud != "" {
		sb.WriteString("\nRead-aloud:\n")
		sb.WriteString(indent(r.ReadAloud))
		sb.WriteString("\n")
	}
	if r.DMNotes != "" {
		sb.WriteString("\nDM notes:\n")
		sb.WriteString(indent(r.DMNotes))
		sb.WriteString("\n")
	}
	if len(r.NPCIDs) > 0 {
		sb.WriteString("\nNPCs present: ")
		names := make([]string, 0, len(r.NPCIDs))
		for _, id := range r.NPCIDs {
			if n := adv.NPC(id); n != nil {
				names = append(names, fmt.Sprintf("%s [%s]", n.Name, n.ID))
			} else {
				names = append(names, id)
			}
		}
		sb.WriteString(strings.Join(names, ", ") + "\n")
	}
	if len(r.Features) > 0 {
		sb.WriteString("\nFeatures:\n")
		for _, f := range r.Features {
			line := "  - " + f.Name
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
			sb.WriteString(line + "\n")
		}
	}
	if len(r.Encounters) > 0 {
		sb.WriteString("\nEncounters:\n")
		for _, e := range r.Encounters {
			sb.WriteString(fmt.Sprintf("  - %s", e.Name))
			if e.Difficulty != "" {
				sb.WriteString(fmt.Sprintf(" [%s]", e.Difficulty))
			}
			sb.WriteString("\n")
			if e.Description != "" {
				sb.WriteString(indent(e.Description) + "\n")
			}
		}
	}
	if len(r.Treasure) > 0 {
		sb.WriteString("\nTreasure: " + strings.Join(r.Treasure, ", ") + "\n")
	}
	if len(r.Exits) > 0 {
		sb.WriteString("\nExits:\n")
		for _, ex := range r.Exits {
			label := ex.Direction
			if label == "" {
				label = "→"
			}
			line := fmt.Sprintf("  - %s to %s", label, ex.To)
			if ex.Locked {
				line += " (locked)"
			}
			if ex.Description != "" {
				line += ": " + ex.Description
			}
			sb.WriteString(line + "\n")
		}
	}
	if r.Image != "" {
		sb.WriteString("\n[art: " + r.Image + "]\n")
	}
	return strings.TrimRight(sb.String(), "\n")
}

// FormatNPC renders an NPC dossier: roleplay guidance and mechanics.
func FormatNPC(n *domain.NPC) string {
	if n == nil {
		return "(unknown NPC)"
	}
	var sb strings.Builder
	fmt.Fprintf(&sb, "NPC: %s [%s]", n.Name, n.ID)
	if n.Role != "" {
		fmt.Fprintf(&sb, " — %s", n.Role)
	}
	sb.WriteString("\n")
	writeField(&sb, "Appearance", n.Appearance)
	writeField(&sb, "Personality", n.Personality)
	writeField(&sb, "Motivations", n.Motivations)
	writeField(&sb, "Secrets", n.Secrets)
	writeField(&sb, "Voice", n.Voice)
	writeField(&sb, "Disposition", n.Disposition)
	if len(n.Knowledge) > 0 {
		sb.WriteString("Knows:\n")
		for _, k := range n.Knowledge {
			sb.WriteString("  - " + k + "\n")
		}
	}
	if len(n.SampleDialogue) > 0 {
		sb.WriteString("Sample dialogue:\n")
		for _, d := range n.SampleDialogue {
			sb.WriteString("  \"" + d + "\"\n")
		}
	}
	if n.StatBlock != nil {
		sb.WriteString(formatStatBlock(n.StatBlock))
	}
	if n.Image != "" {
		sb.WriteString("[art: " + n.Image + "]\n")
	}
	return strings.TrimRight(sb.String(), "\n")
}

func formatStatBlock(sb2 *domain.StatBlock) string {
	var sb strings.Builder
	sb.WriteString("Stats: ")
	parts := []string{}
	if sb2.AC > 0 {
		parts = append(parts, fmt.Sprintf("AC %d", sb2.AC))
	}
	if sb2.MaxHP > 0 {
		parts = append(parts, fmt.Sprintf("HP %d", sb2.MaxHP))
	}
	if sb2.Speed != "" {
		parts = append(parts, "Speed "+sb2.Speed)
	}
	if sb2.CR != "" {
		parts = append(parts, "CR "+sb2.CR)
	}
	sb.WriteString(strings.Join(parts, ", ") + "\n")
	a := sb2.Abilities
	if a != (domain.AbilityScores{}) {
		fmt.Fprintf(&sb, "  STR %d (%s) DEX %d (%s) CON %d (%s) INT %d (%s) WIS %d (%s) CHA %d (%s)\n",
			a.STR, domain.ModifierString(a.STR), a.DEX, domain.ModifierString(a.DEX),
			a.CON, domain.ModifierString(a.CON), a.INT, domain.ModifierString(a.INT),
			a.WIS, domain.ModifierString(a.WIS), a.CHA, domain.ModifierString(a.CHA))
	}
	if len(sb2.Skills) > 0 {
		sb.WriteString("  Skills: " + strings.Join(sb2.Skills, ", ") + "\n")
	}
	for _, t := range sb2.Traits {
		sb.WriteString("  Trait: " + t + "\n")
	}
	for _, act := range sb2.Actions {
		line := "  Action: " + act.Name
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
		sb.WriteString(line + "\n")
	}
	return sb.String()
}

// FormatZone renders a zone overview and its room list.
func FormatZone(z *domain.Zone) string {
	if z == nil {
		return "(unknown zone)"
	}
	var sb strings.Builder
	fmt.Fprintf(&sb, "ZONE: %s [%s]\n", z.Name, z.ID)
	writeField(&sb, "Overview", z.Overview)
	writeField(&sb, "Description", z.Description)
	if len(z.Rooms) > 0 {
		sb.WriteString("Rooms:\n")
		for _, r := range z.Rooms {
			sb.WriteString(fmt.Sprintf("  - %s [%s]\n", r.Name, r.ID))
		}
	}
	if z.MapImage != "" {
		sb.WriteString("[map: " + z.MapImage + "]\n")
	}
	return strings.TrimRight(sb.String(), "\n")
}

// FormatEvent renders a scripted event.
func FormatEvent(e *domain.Event) string {
	if e == nil {
		return "(unknown event)"
	}
	var sb strings.Builder
	fmt.Fprintf(&sb, "EVENT: %s [%s]\n", e.Name, e.ID)
	writeField(&sb, "Trigger", e.Trigger)
	writeField(&sb, "Description", e.Description)
	if e.ReadAloud != "" {
		sb.WriteString("Read-aloud:\n" + indent(e.ReadAloud) + "\n")
	}
	writeField(&sb, "DM notes", e.DMNotes)
	writeField(&sb, "Consequences", e.Consequences)
	for _, o := range e.Outcomes {
		fmt.Fprintf(&sb, "  If %s → %s\n", o.Condition, o.Result)
	}
	return strings.TrimRight(sb.String(), "\n")
}

// FormatItem renders an item entry.
func FormatItem(it *domain.Item) string {
	if it == nil {
		return "(unknown item)"
	}
	var sb strings.Builder
	fmt.Fprintf(&sb, "ITEM: %s [%s]\n", it.Name, it.ID)
	writeField(&sb, "Rarity", it.Rarity)
	writeField(&sb, "Description", it.Description)
	writeField(&sb, "Mechanics", it.Mechanics)
	return strings.TrimRight(sb.String(), "\n")
}

func writeField(sb *strings.Builder, label, value string) {
	if strings.TrimSpace(value) == "" {
		return
	}
	sb.WriteString(label + ": " + value + "\n")
}

func indent(s string) string {
	lines := strings.Split(strings.TrimRight(s, "\n"), "\n")
	for i, l := range lines {
		lines[i] = "  " + l
	}
	return strings.Join(lines, "\n")
}
