package main

import (
	"fmt"
	"strings"

	"github.com/theburrowhub/thaimaturgy/internal/domain"
)

// This file renders authored adventure content as readable Markdown for the GUI
// detail pane (word-wrapped prose, headings, lists) and extracts navigable
// references (navGroup/navRef) rendered as in-app links. It is deliberately
// separate from engine/format.go, whose plain-text output targets the TUI and the
// oracle context, not a rich Markdown widget.

// navRef is a tappable link to another node in the adventure browser.
type navRef struct {
	label string
	uid   string // tree node id, e.g. "room:zone::rid", "npc:id"
}

// navGroup is a titled cluster of related links (e.g. "Exits", "NPCs here").
type navGroup struct {
	title string
	refs  []navRef
}

// --- Markdown renderers --------------------------------------------------

func mdHeading(name, id string) string {
	name = strings.TrimSpace(name)
	if name == "" {
		name = id
	}
	if id != "" {
		return fmt.Sprintf("## %s\n\n`%s`\n\n", name, id)
	}
	return fmt.Sprintf("## %s\n\n", name)
}

// mdField renders a labelled prose field as a bold label followed by its value,
// or nothing when empty.
func mdField(label, value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return ""
	}
	return fmt.Sprintf("**%s**\n\n%s\n\n", label, value)
}

// mdQuote renders multi-line read-aloud text as a Markdown block quote.
func mdQuote(label, value string) string {
	value = strings.TrimRight(value, "\n")
	if strings.TrimSpace(value) == "" {
		return ""
	}
	var sb strings.Builder
	fmt.Fprintf(&sb, "**%s**\n\n", label)
	for _, line := range strings.Split(value, "\n") {
		sb.WriteString("> " + line + "\n")
	}
	sb.WriteString("\n")
	return sb.String()
}

// adventureMarkdown renders the module's front-matter (summary, positioning
// context, DM background, introduction, conclusion, hooks) for the browser's
// "Adventure" node, so the DM can read the trasfondo during play.
func adventureMarkdown(adv *domain.Adventure) string {
	var sb strings.Builder
	sb.WriteString(mdHeading(adv.Title, adv.ID))
	if adv.System != "" {
		sb.WriteString(mdField("System", adv.System))
	}
	sb.WriteString(mdField("Summary", adv.Summary))
	sb.WriteString(mdField("Context", adv.Context))
	sb.WriteString(mdField("Background", adv.Background))
	sb.WriteString(mdField("Introduction", adv.Introduction))
	sb.WriteString(mdField("Conclusion", adv.Conclusion))
	if len(adv.Hooks) > 0 {
		sb.WriteString("**Hooks**\n\n")
		for _, h := range adv.Hooks {
			sb.WriteString("- " + h + "\n")
		}
		sb.WriteString("\n")
	}
	return strings.TrimSpace(sb.String())
}

func zoneMarkdown(z *domain.Zone) string {
	var sb strings.Builder
	sb.WriteString(mdHeading(z.Name, z.ID))
	sb.WriteString(mdField("Overview", z.Overview))
	sb.WriteString(mdField("Description", z.Description))
	return strings.TrimSpace(sb.String())
}

func roomMarkdown(r *domain.Room) string {
	var sb strings.Builder
	sb.WriteString(mdHeading(r.Name, r.ID))
	sb.WriteString(mdQuote("Read-aloud", r.ReadAloud))
	sb.WriteString(mdField("DM notes", r.DMNotes))
	if len(r.Features) > 0 {
		sb.WriteString("**Features**\n\n")
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
			sb.WriteString("- " + line + "\n")
		}
		sb.WriteString("\n")
	}
	if len(r.Encounters) > 0 {
		sb.WriteString("**Encounters**\n\n")
		for _, e := range r.Encounters {
			line := e.Name
			if e.Difficulty != "" {
				line += fmt.Sprintf(" [%s]", e.Difficulty)
			}
			if e.Description != "" {
				line += ": " + e.Description
			}
			sb.WriteString("- " + line + "\n")
		}
		sb.WriteString("\n")
	}
	if len(r.Treasure) > 0 {
		sb.WriteString(mdField("Treasure", strings.Join(r.Treasure, ", ")))
	}
	return strings.TrimSpace(sb.String())
}

func npcMarkdown(n *domain.NPC) string {
	var sb strings.Builder
	title := n.Name
	if n.Role != "" {
		title = fmt.Sprintf("%s — %s", n.Name, n.Role)
	}
	sb.WriteString(mdHeading(title, n.ID))
	sb.WriteString(mdField("Appearance", n.Appearance))
	sb.WriteString(mdField("Personality", n.Personality))
	sb.WriteString(mdField("Motivations", n.Motivations))
	sb.WriteString(mdField("Secrets", n.Secrets))
	sb.WriteString(mdField("Voice", n.Voice))
	sb.WriteString(mdField("Disposition", n.Disposition))
	if len(n.Knowledge) > 0 {
		sb.WriteString("**Knows**\n\n")
		for _, k := range n.Knowledge {
			sb.WriteString("- " + k + "\n")
		}
		sb.WriteString("\n")
	}
	if len(n.SampleDialogue) > 0 {
		sb.WriteString("**Sample dialogue**\n\n")
		for _, d := range n.SampleDialogue {
			sb.WriteString("> " + d + "\n")
		}
		sb.WriteString("\n")
	}
	sb.WriteString(statBlockMarkdown(n.StatBlock))
	return strings.TrimSpace(sb.String())
}

func statBlockMarkdown(s *domain.StatBlock) string {
	if s == nil {
		return ""
	}
	var sb strings.Builder
	sb.WriteString("**Stats**\n\n")
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
		sb.WriteString(strings.Join(parts, " · ") + "\n\n")
	}
	if a := s.Abilities; a != (domain.AbilityScores{}) {
		fmt.Fprintf(&sb, "STR %d (%s) · DEX %d (%s) · CON %d (%s) · INT %d (%s) · WIS %d (%s) · CHA %d (%s)\n\n",
			a.STR, domain.ModifierString(a.STR), a.DEX, domain.ModifierString(a.DEX),
			a.CON, domain.ModifierString(a.CON), a.INT, domain.ModifierString(a.INT),
			a.WIS, domain.ModifierString(a.WIS), a.CHA, domain.ModifierString(a.CHA))
	}
	if len(s.Skills) > 0 {
		sb.WriteString("Skills: " + strings.Join(s.Skills, ", ") + "\n\n")
	}
	for _, t := range s.Traits {
		sb.WriteString("- *Trait:* " + t + "\n")
	}
	for _, act := range s.Actions {
		line := "*" + act.Name + "*"
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
		sb.WriteString("- " + line + "\n")
	}
	return sb.String()
}

func eventMarkdown(e *domain.Event) string {
	var sb strings.Builder
	sb.WriteString(mdHeading(e.Name, e.ID))
	sb.WriteString(mdField("Trigger", e.Trigger))
	sb.WriteString(mdField("Description", e.Description))
	sb.WriteString(mdQuote("Read-aloud", e.ReadAloud))
	sb.WriteString(mdField("DM notes", e.DMNotes))
	sb.WriteString(mdField("Consequences", e.Consequences))
	if len(e.Outcomes) > 0 {
		sb.WriteString("**Outcomes**\n\n")
		for _, o := range e.Outcomes {
			sb.WriteString(fmt.Sprintf("- If %s → %s\n", o.Condition, o.Result))
		}
		sb.WriteString("\n")
	}
	return strings.TrimSpace(sb.String())
}

func itemMarkdown(it *domain.Item) string {
	var sb strings.Builder
	sb.WriteString(mdHeading(it.Name, it.ID))
	sb.WriteString(mdField("Rarity", it.Rarity))
	sb.WriteString(mdField("Description", it.Description))
	sb.WriteString(mdField("Mechanics", it.Mechanics))
	return strings.TrimSpace(sb.String())
}

// --- Navigation references -----------------------------------------------

func zoneGroups(adv *domain.Adventure, z *domain.Zone) []navGroup {
	var rooms []navRef
	for _, r := range z.Rooms {
		rooms = append(rooms, navRef{labelOrID(r.Name, r.ID), "room:" + z.ID + "::" + r.ID})
	}
	var conns []navRef
	for _, cid := range z.Connections {
		if zz := adv.Zone(cid); zz != nil {
			conns = append(conns, navRef{labelOrID(zz.Name, zz.ID), "zone:" + zz.ID})
		}
	}
	return []navGroup{{"Rooms", rooms}, {"Connects to", conns}}
}

func roomGroups(adv *domain.Adventure, r *domain.Room) []navGroup {
	var exits, npcs, events []navRef
	for _, ex := range r.Exits {
		if ex.To == "" {
			continue
		}
		if rr, zz := adv.Room(ex.To); rr != nil && zz != nil {
			label := labelOrID(rr.Name, rr.ID)
			if ex.Direction != "" {
				label = ex.Direction + " → " + label
			}
			if ex.Locked {
				label += " (locked)"
			}
			exits = append(exits, navRef{label, "room:" + zz.ID + "::" + rr.ID})
		} else if z := adv.Zone(ex.To); z != nil {
			exits = append(exits, navRef{"→ " + labelOrID(z.Name, z.ID), "zone:" + z.ID})
		}
	}
	for _, id := range r.NPCIDs {
		if n := adv.NPC(id); n != nil {
			npcs = append(npcs, navRef{labelOrID(n.Name, n.ID), "npc:" + n.ID})
		}
	}
	for _, id := range r.EventIDs {
		if e := adv.Event(id); e != nil {
			events = append(events, navRef{labelOrID(e.Name, e.ID), "event:" + e.ID})
		}
	}
	return []navGroup{{"Exits", exits}, {"NPCs here", npcs}, {"Events", events}}
}

func npcGroups(adv *domain.Adventure, n *domain.NPC) []navGroup {
	if n.DefaultLocation == "" {
		return nil
	}
	if r, z := adv.Room(n.DefaultLocation); r != nil && z != nil {
		return []navGroup{{"Location", []navRef{{labelOrID(r.Name, r.ID), "room:" + z.ID + "::" + r.ID}}}}
	}
	return nil
}
