package main

import (
	"fmt"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"

	"github.com/theburrowhub/thaimaturgy/internal/domain"
)

// showRoster opens the persistent campaign roster (issue #33): the pool of player
// characters that outlive a session. It lists saved characters (each can be added
// to the current party or deleted) and can save the current party back to the
// roster, so characters carry their progression across adventures.
func (g *gui) showRoster() {
	if g.session == nil {
		return
	}

	list := container.NewVBox()
	status := widget.NewLabel("")
	status.Wrapping = fyne.TextWrapWord

	var pop *widget.PopUp
	var refresh func()

	addToParty := func(id string) {
		c, err := g.store.LoadCharacter(id)
		if err != nil {
			status.SetText("⚠ " + err.Error())
			return
		}
		party := partyPointers(g.session.State.PartySnapshot())
		for _, m := range party {
			if m.ID == c.ID {
				status.SetText(c.Name + " is already in the party.")
				return
			}
		}
		party = append(party, c)
		// SetParty runs EnsureUniqueNames, so if a member with the same name is
		// already in the party the newcomer is renamed (e.g. "Bob 2") — party
		// members stay uniquely addressable by name (/pick, MutateCharacter).
		g.session.State.SetParty(party)
		g.refreshPCPanel()
		g.autosave()
		msg := "Added " + c.Name + " to the party."
		if c.Name != "" {
			for _, m := range g.session.State.PartySnapshot() {
				if m.ID == c.ID && m.Name != c.Name {
					msg = "Added " + c.Name + " as “" + m.Name + "” (renamed to keep party names unique)."
					break
				}
			}
		}
		status.SetText(msg)
	}

	deleteChar := func(id, name string) {
		if err := g.store.DeleteCharacter(id); err != nil {
			status.SetText("⚠ " + err.Error())
			return
		}
		status.SetText("Deleted " + name + " from the roster.")
		refresh()
	}

	refresh = func() {
		// ListCharacters may return decoded characters AND an error (some files
		// unreadable); surface the warning but still show what loaded.
		chars, err := g.store.ListCharacters()
		objs := []fyne.CanvasObject{}
		if err != nil {
			objs = append(objs, wrapLabel("⚠ "+err.Error()))
		}
		if len(chars) == 0 && err == nil {
			objs = append(objs, widget.NewLabelWithStyle("The roster is empty.", fyne.TextAlignLeading, fyne.TextStyle{Italic: true}),
				wrapLabel("Use “Save current party → roster” to add the current characters."))
		}
		for _, c := range chars {
			id, name := c.ID, c.Name
			label := widget.NewLabel(fmt.Sprintf("%s — Lvl %d %s %s", c.Name, c.Level, c.Race, c.Class))
			add := widget.NewButtonWithIcon("Add to party", theme.ContentAddIcon(), func() { addToParty(id) })
			del := widget.NewButtonWithIcon("", theme.DeleteIcon(), func() { deleteChar(id, name) })
			objs = append(objs, container.NewBorder(nil, nil, nil, container.NewHBox(add, del), label))
		}
		list.Objects = objs
		list.Refresh()
	}
	refresh()

	saveParty := widget.NewButtonWithIcon("Save current party → roster", theme.DocumentSaveIcon(), func() {
		saved := 0
		for _, snap := range g.session.State.PartySnapshot() {
			c := snap
			id, err := g.store.SaveCharacter(&c)
			if err != nil {
				status.SetText("⚠ " + err.Error())
				return
			}
			// Link the live party member so future progression writes back here.
			if snap.ID == "" {
				g.session.State.MutateCharacter(snap.Name, func(lc *domain.Character) {
					if lc.ID == "" {
						lc.ID = id
					}
				})
			}
			saved++
		}
		g.autosave()
		refresh()
		status.SetText(fmt.Sprintf("Saved %d character(s) to the roster.", saved))
	})

	closeBtn := widget.NewButton("Close", func() { pop.Hide() })

	content := container.NewBorder(
		container.NewVBox(
			widget.NewLabelWithStyle("Campaign roster", fyne.TextAlignCenter, fyne.TextStyle{Bold: true}),
			widget.NewSeparator(),
			saveParty,
			widget.NewSeparator(),
		),
		container.NewVBox(status, closeBtn),
		nil, nil,
		container.NewVScroll(list),
	)

	pop = widget.NewModalPopUp(container.NewPadded(content), g.win.Canvas())
	pop.Resize(fyne.NewSize(520, 560))
	pop.Show()
}

// partyPointers converts a race-safe party snapshot into fresh pointers, so the
// caller can build a new party (SetParty replaces wholesale).
func partyPointers(snap []domain.Character) []*domain.Character {
	out := make([]*domain.Character, 0, len(snap))
	for i := range snap {
		c := snap[i]
		out = append(out, &c)
	}
	return out
}
