package wailsapp

import (
	"fmt"
	"strings"

	"github.com/theburrowhub/thaimaturgy/internal/domain"
	"github.com/theburrowhub/thaimaturgy/internal/engine"
)

type NavRef struct {
	Label string `json:"label"`
	UID   string `json:"uid"`
}

type NavGroup struct {
	Title string   `json:"title"`
	Refs  []NavRef `json:"refs"`
}

type DetailPayload struct {
	UID      string     `json:"uid"`
	Kind     string     `json:"kind"`
	Title    string     `json:"title"`
	Markdown string     `json:"markdown"`
	Images   []string   `json:"images,omitempty"`
	Groups   []NavGroup `json:"groups,omitempty"`
	Actions  []string   `json:"actions,omitempty"`
}

type TreeNode struct {
	UID      string     `json:"uid"`
	Label    string     `json:"label"`
	Kind     string     `json:"kind"`
	Children []TreeNode `json:"children,omitempty"`
}

func buildAdventureTree(adv *domain.Adventure, st *domain.SessionState) []TreeNode {
	if adv == nil {
		return nil
	}
	root := []TreeNode{{UID: "about", Label: "Adventure", Kind: "about"}}
	zones := TreeNode{UID: "zones", Label: "Zones", Kind: "section"}
	for _, z := range adv.Zones {
		zn := TreeNode{UID: "zone:" + z.ID, Label: "▸ " + labelOrID(z.Name, z.ID), Kind: "zone"}
		for _, r := range z.Rooms {
			marker := "·"
			if st != nil && st.CurrentRoom == r.ID {
				marker = "● Party"
			} else if st != nil && st.VisitedRooms[r.ID] {
				marker = "✓"
			}
			zn.Children = append(zn.Children, TreeNode{UID: "room:" + z.ID + "::" + r.ID, Label: marker + "  " + labelOrID(r.Name, r.ID), Kind: "room"})
		}
		zones.Children = append(zones.Children, zn)
	}
	root = append(root, zones)
	npcs := TreeNode{UID: "npcs", Label: "NPCs", Kind: "section"}
	for _, n := range adv.NPCs {
		label := labelOrID(n.Name, n.ID)
		if st != nil {
			if s := st.KnownNPCs[n.ID]; s != nil && s.Met {
				label = "✓ " + label
			}
		}
		npcs.Children = append(npcs.Children, TreeNode{UID: "npc:" + n.ID, Label: label, Kind: "npc"})
	}
	root = append(root, npcs)
	events := TreeNode{UID: "events", Label: "Events", Kind: "section"}
	for _, ev := range adv.Events {
		label := labelOrID(ev.Name, ev.ID)
		if st != nil && st.TriggeredEvents[ev.ID] {
			label = "✓ " + label
		}
		events.Children = append(events.Children, TreeNode{UID: "event:" + ev.ID, Label: label, Kind: "event"})
	}
	root = append(root, events)
	items := TreeNode{UID: "items", Label: "Items", Kind: "section"}
	for _, it := range adv.Items {
		items.Children = append(items.Children, TreeNode{UID: "item:" + it.ID, Label: labelOrID(it.Name, it.ID), Kind: "item"})
	}
	root = append(root, items)
	tables := TreeNode{UID: "tables", Label: "Random tables", Kind: "section"}
	for _, tbl := range adv.Tables {
		tables.Children = append(tables.Children, TreeNode{UID: "table:" + tbl.ID, Label: labelOrID(tbl.Name, tbl.ID), Kind: "table"})
	}
	root = append(root, tables)
	return root
}

func detailForUID(adv *domain.Adventure, st *domain.SessionState, uid string) *DetailPayload {
	if adv == nil {
		return &DetailPayload{UID: uid, Markdown: "_No adventure loaded._"}
	}
	if strings.TrimSpace(uid) == "" {
		uid = "about"
	}
	d := &DetailPayload{UID: uid}
	switch {
	case uid == "about":
		d.Kind, d.Title, d.Markdown = "about", adv.Title, adventureMarkdown(adv)
	case strings.HasPrefix(uid, "zone:"):
		id := strings.TrimPrefix(uid, "zone:")
		if z := adv.Zone(id); z != nil {
			d.Kind, d.Title, d.Markdown = "zone", labelOrID(z.Name, z.ID), zoneMarkdown(z)
			d.Groups = zoneGroups(adv, z)
			d.Images = adv.ZoneImages(z)
		}
	case strings.HasPrefix(uid, "room:"):
		_, rid := splitRoomUID(uid)
		if r, _ := adv.Room(rid); r != nil {
			d.Kind, d.Title, d.Markdown = "room", labelOrID(r.Name, r.ID), roomMarkdown(r)
			d.Groups = roomGroups(adv, r)
			d.Images = adv.RoomImages(r)
			d.Actions = []string{"move"}
		}
	case strings.HasPrefix(uid, "npc:"):
		id := strings.TrimPrefix(uid, "npc:")
		if n := adv.NPC(id); n != nil {
			d.Kind, d.Title, d.Markdown = "npc", labelOrID(n.Name, n.ID), npcMarkdown(n)
			d.Groups = npcGroups(adv, n)
			d.Images = adv.NPCImages(n)
			d.Actions = []string{"mark_npc_met"}
		}
	case strings.HasPrefix(uid, "event:"):
		id := strings.TrimPrefix(uid, "event:")
		if ev := adv.Event(id); ev != nil {
			d.Kind, d.Title, d.Markdown = "event", labelOrID(ev.Name, ev.ID), eventMarkdown(ev)
			d.Actions = []string{"trigger_event"}
		}
	case strings.HasPrefix(uid, "item:"):
		id := strings.TrimPrefix(uid, "item:")
		if it := adv.Item(id); it != nil {
			d.Kind, d.Title, d.Markdown = "item", labelOrID(it.Name, it.ID), itemMarkdown(it)
			d.Images = adv.ItemImages(it)
		}
	case strings.HasPrefix(uid, "table:"):
		id := strings.TrimPrefix(uid, "table:")
		if t := adv.Table(id); t != nil {
			d.Kind, d.Title = "table", labelOrID(t.Name, t.ID)
			d.Markdown = "## " + labelOrID(t.Name, t.ID) + "\n\n" + engine.TableMarkdown(t)
			if len(t.Rows) > 0 {
				d.Actions = []string{"roll_table"}
			}
		}
	}
	if d.Markdown == "" {
		d.Markdown = "_Select a zone, room, NPC, event, item or table._"
	}
	return d
}

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
func mdField(label, value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return ""
	}
	return fmt.Sprintf("**%s**\n\n%s\n\n", label, value)
}
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
func adventureMarkdown(adv *domain.Adventure) string {
	var sb strings.Builder
	sb.WriteString(mdHeading(adv.Title, adv.ID))
	sb.WriteString(mdField("System", adv.System))
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
	return strings.TrimSpace(mdHeading(z.Name, z.ID) + mdField("Overview", z.Overview) + mdField("Description", z.Description))
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
		fmt.Fprintf(&sb, "STR %d (%s) · DEX %d (%s) · CON %d (%s) · INT %d (%s) · WIS %d (%s) · CHA %d (%s)\n\n", a.STR, domain.ModifierString(a.STR), a.DEX, domain.ModifierString(a.DEX), a.CON, domain.ModifierString(a.CON), a.INT, domain.ModifierString(a.INT), a.WIS, domain.ModifierString(a.WIS), a.CHA, domain.ModifierString(a.CHA))
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
	return strings.TrimSpace(mdHeading(it.Name, it.ID) + mdField("Rarity", it.Rarity) + mdField("Description", it.Description) + mdField("Mechanics", it.Mechanics))
}
func zoneGroups(adv *domain.Adventure, z *domain.Zone) []NavGroup {
	var rooms, conns []NavRef
	for _, r := range z.Rooms {
		rooms = append(rooms, NavRef{labelOrID(r.Name, r.ID), "room:" + z.ID + "::" + r.ID})
	}
	for _, cid := range z.Connections {
		if zz := adv.Zone(cid); zz != nil {
			conns = append(conns, NavRef{labelOrID(zz.Name, zz.ID), "zone:" + zz.ID})
		}
	}
	return []NavGroup{{"Rooms", rooms}, {"Connects to", conns}}
}
func roomGroups(adv *domain.Adventure, r *domain.Room) []NavGroup {
	var exits, npcs, events []NavRef
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
			exits = append(exits, NavRef{label, "room:" + zz.ID + "::" + rr.ID})
		} else if z := adv.Zone(ex.To); z != nil {
			exits = append(exits, NavRef{"→ " + labelOrID(z.Name, z.ID), "zone:" + z.ID})
		}
	}
	for _, id := range r.NPCIDs {
		if n := adv.NPC(id); n != nil {
			npcs = append(npcs, NavRef{labelOrID(n.Name, n.ID), "npc:" + n.ID})
		}
	}
	for _, id := range r.EventIDs {
		if e := adv.Event(id); e != nil {
			events = append(events, NavRef{labelOrID(e.Name, e.ID), "event:" + e.ID})
		}
	}
	return []NavGroup{{"Exits", exits}, {"NPCs here", npcs}, {"Events", events}}
}
func npcGroups(adv *domain.Adventure, n *domain.NPC) []NavGroup {
	var rooms []NavRef
	if n.DefaultLocation != "" {
		if r, z := adv.Room(n.DefaultLocation); r != nil && z != nil {
			rooms = append(rooms, NavRef{labelOrID(r.Name, r.ID), "room:" + z.ID + "::" + r.ID})
		}
	}
	return []NavGroup{{"Default location", rooms}}
}
func splitRoomUID(uid string) (zoneID, roomID string) {
	body := strings.TrimPrefix(uid, "room:")
	if i := strings.Index(body, "::"); i >= 0 {
		return body[:i], body[i+2:]
	}
	return "", body
}
func labelOrID(name, id string) string {
	if strings.TrimSpace(name) != "" {
		return name
	}
	return id
}
