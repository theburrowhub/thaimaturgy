package main

import (
	"strconv"
	"strings"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/widget"

	"github.com/theburrowhub/thaimaturgy/internal/domain"
)

// --- Forms ---------------------------------------------------------------

func (e *editor) metaForm() fyne.CanvasObject {
	a := e.adv
	return container.NewVBox(
		heading("Adventure"),
		field("ID", e.sEntry(&a.ID)),
		field("Title", e.treeStr(&a.Title, "", false)),
		field("Author", e.sEntry(&a.Author)),
		field("System", e.sEntry(&a.System)),
		field("Language (en/es)", e.sEntry(&a.Language)),
		field("Summary", e.mEntry(&a.Summary)),
		field("Background (DM-only)", e.mEntry(&a.Background)),
		field("Introduction", e.mEntry(&a.Introduction)),
		field("Conclusion", e.mEntry(&a.Conclusion)),
		field("Hooks (one per line)", e.listEntry(&a.Hooks)),
	)
}

func (e *editor) zoneForm(z *domain.Zone) fyne.CanvasObject {
	return container.NewVBox(
		heading("Zone"),
		field("ID", e.treeStr(&z.ID, "zone:", true)),
		field("Name", e.treeStr(&z.Name, "", false)),
		field("Overview (DM)", e.mEntry(&z.Overview)),
		field("Description", e.mEntry(&z.Description)),
		field("Map image", e.imageField("maps", &z.MapImage)),
		field("Connections — zone IDs (one per line)", e.listEntry(&z.Connections)),
		widget.NewLabel("Rooms are listed under this zone in the tree. Use +Room to add."),
	)
}

func (e *editor) roomForm(r *domain.Room) fyne.CanvasObject {
	zid := e.selectedZoneID()
	prefix := "room:" + zid + "::"
	return container.NewVBox(
		heading("Room"),
		field("ID", e.treeStr(&r.ID, prefix, true)),
		field("Name", e.treeStr(&r.Name, "", false)),
		field("Read-aloud text", e.mEntry(&r.ReadAloud)),
		field("DM notes", e.mEntry(&r.DMNotes)),
		field("Image", e.imageField("art", &r.Image)),
		field("NPC IDs (one per line)", e.listEntry(&r.NPCIDs)),
		field("Event IDs (one per line)", e.listEntry(&r.EventIDs)),
		field("Treasure (one per line)", e.listEntry(&r.Treasure)),
		e.exitsEditor(r),
		e.featuresEditor(r),
		e.encountersEditor(r),
	)
}

func (e *editor) npcForm(n *domain.NPC) fyne.CanvasObject {
	box := container.NewVBox(
		heading("NPC"),
		field("ID", e.treeStr(&n.ID, "npc:", true)),
		field("Name", e.treeStr(&n.Name, "", false)),
		field("Role", e.sEntry(&n.Role)),
		field("Appearance", e.mEntry(&n.Appearance)),
		field("Personality", e.mEntry(&n.Personality)),
		field("Motivations", e.mEntry(&n.Motivations)),
		field("Secrets", e.mEntry(&n.Secrets)),
		field("Voice", e.sEntry(&n.Voice)),
		field("Disposition", e.sEntry(&n.Disposition)),
		field("Default location (room ID)", e.sEntry(&n.DefaultLocation)),
		field("Image", e.imageField("art", &n.Image)),
		field("Knowledge (one per line)", e.listEntry(&n.Knowledge)),
		field("Sample dialogue (one per line)", e.listEntry(&n.SampleDialogue)),
	)
	box.Add(e.statBlockEditor(n))
	return box
}

func (e *editor) statBlockEditor(n *domain.NPC) fyne.CanvasObject {
	if n.StatBlock == nil {
		return container.NewVBox(
			widget.NewSeparator(),
			widget.NewButton("+ Add stat block", func() {
				n.StatBlock = &domain.StatBlock{}
				e.dirty = true
				e.refreshForm()
			}),
		)
	}
	sb := n.StatBlock
	ab := &sb.Abilities
	abilities := container.NewGridWithColumns(6,
		field("STR", e.iEntry(&ab.STR)), field("DEX", e.iEntry(&ab.DEX)), field("CON", e.iEntry(&ab.CON)),
		field("INT", e.iEntry(&ab.INT)), field("WIS", e.iEntry(&ab.WIS)), field("CHA", e.iEntry(&ab.CHA)),
	)
	return container.NewVBox(
		widget.NewSeparator(),
		heading("Stat block"),
		container.NewGridWithColumns(4,
			field("AC", e.iEntry(&sb.AC)), field("Max HP", e.iEntry(&sb.MaxHP)),
			field("Speed", e.sEntry(&sb.Speed)), field("CR", e.sEntry(&sb.CR)),
		),
		abilities,
		field("Skills (one per line)", e.listEntry(&sb.Skills)),
		field("Traits (one per line)", e.listEntry(&sb.Traits)),
		e.actionsEditor(sb),
		widget.NewButton("Remove stat block", func() {
			n.StatBlock = nil
			e.dirty = true
			e.refreshForm()
		}),
	)
}

func (e *editor) eventForm(ev *domain.Event) fyne.CanvasObject {
	return container.NewVBox(
		heading("Event"),
		field("ID", e.treeStr(&ev.ID, "event:", true)),
		field("Name", e.treeStr(&ev.Name, "", false)),
		field("Trigger", e.mEntry(&ev.Trigger)),
		field("Description", e.mEntry(&ev.Description)),
		field("Read-aloud text", e.mEntry(&ev.ReadAloud)),
		field("DM notes", e.mEntry(&ev.DMNotes)),
		field("Consequences", e.mEntry(&ev.Consequences)),
		e.outcomesEditor(ev),
	)
}

func (e *editor) itemForm(it *domain.Item) fyne.CanvasObject {
	return container.NewVBox(
		heading("Item"),
		field("ID", e.treeStr(&it.ID, "item:", true)),
		field("Name", e.treeStr(&it.Name, "", false)),
		field("Rarity", e.sEntry(&it.Rarity)),
		field("Description", e.mEntry(&it.Description)),
		field("Mechanics", e.mEntry(&it.Mechanics)),
		field("Image", e.imageField("art", &it.Image)),
	)
}

// --- Sub-list editors ----------------------------------------------------

func (e *editor) exitsEditor(r *domain.Room) fyne.CanvasObject {
	return e.rows("Exits", len(r.Exits),
		func(i int) fyne.CanvasObject {
			ex := &r.Exits[i]
			return container.NewVBox(
				field("to (room/zone ID)", e.sEntry(&ex.To)),
				field("direction", e.sEntry(&ex.Direction)),
				field("description", e.sEntry(&ex.Description)),
				e.bCheck(&ex.Locked, "locked"),
			)
		},
		func() { r.Exits = append(r.Exits, domain.Exit{}) },
		func(i int) { r.Exits = append(r.Exits[:i], r.Exits[i+1:]...) },
	)
}

func (e *editor) featuresEditor(r *domain.Room) fyne.CanvasObject {
	return e.rows("Features (traps/puzzles/checks)", len(r.Features),
		func(i int) fyne.CanvasObject {
			f := &r.Features[i]
			return container.NewVBox(
				field("name", e.sEntry(&f.Name)),
				field("description", e.mEntry(&f.Description)),
				container.NewGridWithColumns(2, field("skill", e.sEntry(&f.Skill)), field("DC", e.iEntry(&f.DC))),
				field("success", e.mEntry(&f.Success)),
				field("failure", e.mEntry(&f.Failure)),
			)
		},
		func() { r.Features = append(r.Features, domain.Feature{}) },
		func(i int) { r.Features = append(r.Features[:i], r.Features[i+1:]...) },
	)
}

func (e *editor) encountersEditor(r *domain.Room) fyne.CanvasObject {
	return e.rows("Encounters", len(r.Encounters),
		func(i int) fyne.CanvasObject {
			en := &r.Encounters[i]
			return container.NewVBox(
				field("name", e.sEntry(&en.Name)),
				field("difficulty", e.sEntry(&en.Difficulty)),
				field("description", e.mEntry(&en.Description)),
				field("tactics", e.mEntry(&en.Tactics)),
				field("creatures (one per line)", e.listEntry(&en.Creatures)),
			)
		},
		func() { r.Encounters = append(r.Encounters, domain.Encounter{}) },
		func(i int) { r.Encounters = append(r.Encounters[:i], r.Encounters[i+1:]...) },
	)
}

func (e *editor) actionsEditor(sb *domain.StatBlock) fyne.CanvasObject {
	return e.rows("Actions", len(sb.Actions),
		func(i int) fyne.CanvasObject {
			a := &sb.Actions[i]
			return container.NewVBox(
				field("name", e.sEntry(&a.Name)),
				container.NewGridWithColumns(2, field("to hit", e.sEntry(&a.ToHit)), field("damage", e.sEntry(&a.Damage))),
				field("description", e.mEntry(&a.Description)),
			)
		},
		func() { sb.Actions = append(sb.Actions, domain.Action{}) },
		func(i int) { sb.Actions = append(sb.Actions[:i], sb.Actions[i+1:]...) },
	)
}

func (e *editor) outcomesEditor(ev *domain.Event) fyne.CanvasObject {
	return e.rows("Outcomes (branches)", len(ev.Outcomes),
		func(i int) fyne.CanvasObject {
			o := &ev.Outcomes[i]
			return container.NewVBox(
				field("condition", e.sEntry(&o.Condition)),
				field("result", e.mEntry(&o.Result)),
			)
		},
		func() { ev.Outcomes = append(ev.Outcomes, domain.Outcome{}) },
		func(i int) { ev.Outcomes = append(ev.Outcomes[:i], ev.Outcomes[i+1:]...) },
	)
}

// rows renders a dynamic list with per-row Remove and a trailing Add button.
func (e *editor) rows(title string, n int, render func(i int) fyne.CanvasObject, add func(), remove func(i int)) fyne.CanvasObject {
	box := container.NewVBox(widget.NewSeparator(), heading(title))
	for i := 0; i < n; i++ {
		idx := i
		rm := widget.NewButton("✕ Remove", func() {
			remove(idx)
			e.dirty = true
			e.refreshForm()
		})
		box.Add(container.NewVBox(render(idx), rm, widget.NewSeparator()))
	}
	box.Add(widget.NewButton("+ Add", func() {
		add()
		e.dirty = true
		e.refreshForm()
	}))
	return box
}

// --- Bound widgets -------------------------------------------------------

func (e *editor) sEntry(p *string) *widget.Entry {
	ent := widget.NewEntry()
	ent.SetText(*p)
	ent.OnChanged = func(s string) { *p = s; e.dirty = true }
	return ent
}

func (e *editor) mEntry(p *string) *widget.Entry {
	ent := widget.NewMultiLineEntry()
	ent.Wrapping = fyne.TextWrapWord
	ent.SetText(*p)
	ent.SetMinRowsVisible(3)
	ent.OnChanged = func(s string) { *p = s; e.dirty = true }
	return ent
}

func (e *editor) iEntry(p *int) *widget.Entry {
	ent := widget.NewEntry()
	ent.SetText(strconv.Itoa(*p))
	ent.OnChanged = func(s string) {
		s = strings.TrimSpace(s)
		if s == "" {
			*p = 0
			e.dirty = true
			return
		}
		if v, err := strconv.Atoi(s); err == nil {
			*p = v
			e.dirty = true
		}
	}
	return ent
}

func (e *editor) bCheck(p *bool, label string) *widget.Check {
	c := widget.NewCheck(label, func(b bool) { *p = b; e.dirty = true })
	c.SetChecked(*p)
	return c
}

func (e *editor) listEntry(p *[]string) *widget.Entry {
	ent := widget.NewMultiLineEntry()
	ent.Wrapping = fyne.TextWrapWord
	ent.SetText(strings.Join(*p, "\n"))
	ent.OnChanged = func(s string) { *p = splitLines(s); e.dirty = true }
	return ent
}

// treeStr edits a struct field that appears in the navigation tree, refreshing
// the tree as it changes. When isID is true it also keeps currentUID in sync so
// the form can be rebuilt after the ID changes.
func (e *editor) treeStr(p *string, uidPrefix string, isID bool) *widget.Entry {
	ent := widget.NewEntry()
	ent.SetText(*p)
	ent.OnChanged = func(s string) {
		*p = s
		e.dirty = true
		if isID {
			e.currentUID = uidPrefix + s
		}
		e.refreshTree()
	}
	return ent
}

func (e *editor) imageField(kind string, p *string) fyne.CanvasObject {
	ent := e.sEntry(p)
	btn := widget.NewButton("Import…", func() {
		e.importImage(kind, func(rel string) { *p = rel; e.refreshForm() })
	})
	return container.NewBorder(nil, nil, nil, btn, ent)
}

// --- small helpers -------------------------------------------------------

func heading(s string) fyne.CanvasObject {
	return widget.NewLabelWithStyle(s, fyne.TextAlignLeading, fyne.TextStyle{Bold: true})
}

func field(label string, w fyne.CanvasObject) fyne.CanvasObject {
	return container.NewVBox(widget.NewLabel(label), w)
}

func splitLines(s string) []string {
	var out []string
	for _, ln := range strings.Split(s, "\n") {
		ln = strings.TrimRight(ln, "\r")
		if strings.TrimSpace(ln) != "" {
			out = append(out, ln)
		}
	}
	return out
}
