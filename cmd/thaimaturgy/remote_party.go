package main

import (
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"

	"github.com/theburrowhub/thaimaturgy/internal/domain"
)

// remotePartyPanel builds the editable party panel for the remote GUI (#130):
// the member sheets with a per-member Remove, plus a toolbar to add from the
// roster or set the default party. Every mutation goes through the HTTP API and
// then refetches so the panel reflects the server. (AI party-plan and full 5e
// sheet editing are follow-ups.)
func (g *gui) remotePartyPanel(name string, initial *domain.SessionState) (fyne.CanvasObject, func()) {
	list := container.NewVBox()
	listScroll := container.NewVScroll(list)

	var refresh func()
	refresh = func() {
		go func() {
			ctx, cancel := bg(15)
			members, err := g.remote.Party(ctx, name)
			cancel()
			fyne.Do(func() {
				if err != nil {
					g.showErr(err)
					return
				}
				g.fillRemoteParty(list, members, name, refresh)
			})
		}()
	}

	// Seed from the snapshot we already have, then keep in sync via refresh.
	if initial != nil {
		g.fillRemoteParty(list, initial.PartySnapshot(), name, refresh)
	}

	addBtn := widget.NewButtonWithIcon("From roster…", theme.ContentAddIcon(), func() {
		g.remoteAddFromRoster(name, refresh)
	})
	defBtn := widget.NewButtonWithIcon("Default party", theme.AccountIcon(), func() {
		go func() {
			ctx, cancel := bg(15)
			err := g.remote.DefaultParty(ctx, name)
			cancel()
			fyne.Do(func() {
				if err != nil {
					g.showErr(err)
					return
				}
				refresh()
			})
		}()
	})
	toolbar := container.NewHBox(addBtn, defBtn)
	return container.NewBorder(toolbar, nil, nil, nil, listScroll), refresh
}

// fillRemoteParty renders the party members with a Remove button each.
func (g *gui) fillRemoteParty(list *fyne.Container, members []domain.Character, name string, refresh func()) {
	list.Objects = nil
	if len(members) == 0 {
		list.Add(widget.NewLabelWithStyle("No party.", fyne.TextAlignLeading, fyne.TextStyle{Italic: true}))
		list.Refresh()
		return
	}
	for i := range members {
		m := members[i] // capture a stable copy for the closure + &m in buildPCSheet
		remove := widget.NewButtonWithIcon("", theme.DeleteIcon(), func() {
			g.remoteRemoveMember(name, m, refresh)
		})
		remove.Importance = widget.LowImportance
		title := widget.NewLabelWithStyle(m.Name, fyne.TextAlignLeading, fyne.TextStyle{Bold: true})
		list.Add(container.NewBorder(nil, nil, nil, remove, title))
		list.Objects = append(list.Objects, buildPCSheet(&m)...)
	}
	list.Refresh()
}

// remoteAddFromRoster lists roster characters and adds the chosen one to the
// party (skipping one already present by id, mirroring the web dedup guard).
func (g *gui) remoteAddFromRoster(name string, refresh func()) {
	go func() {
		ctx, cancel := bg(15)
		roster, rerr := g.remote.ListCharacters(ctx)
		members, perr := g.remote.Party(ctx, name)
		cancel()
		fyne.Do(func() {
			if rerr != nil {
				g.showErr(rerr)
				return
			}
			if perr != nil {
				g.showErr(perr)
				return
			}
			inParty := make(map[string]bool, len(members))
			for i := range members {
				if members[i].ID != "" {
					inParty[members[i].ID] = true
				}
			}
			body := container.NewVBox()
			var pop *widget.PopUp
			if len(roster) == 0 {
				body.Add(widget.NewLabel("The roster is empty."))
			}
			for i := range roster {
				rc := roster[i]
				row := widget.NewLabel(rc.Name + " — " + rc.Race + " " + rc.Class)
				add := widget.NewButtonWithIcon("Add", theme.ContentAddIcon(), func() {
					if rc.ID != "" && inParty[rc.ID] {
						g.showErr(errString(rc.Name + " is already in the party."))
						return
					}
					pop.Hide()
					g.remoteAppendMember(name, rc, refresh)
				})
				body.Add(container.NewBorder(nil, nil, nil, add, row))
			}
			body.Add(widget.NewButton("Close", func() { pop.Hide() }))
			pop = widget.NewModalPopUp(container.NewPadded(container.NewVScroll(body)), g.win.Canvas())
			pop.Resize(fyne.NewSize(420, 480))
			pop.Show()
		})
	}()
}

// remoteAppendMember appends one character to the party and saves it.
func (g *gui) remoteAppendMember(name string, add *domain.Character, refresh func()) {
	go func() {
		ctx, cancel := bg(15)
		members, err := g.remote.Party(ctx, name)
		if err == nil {
			next := make([]*domain.Character, 0, len(members)+1)
			for i := range members {
				m := members[i]
				next = append(next, &m)
			}
			next = append(next, add)
			err = g.remote.SetParty(ctx, name, next)
		}
		cancel()
		fyne.Do(func() {
			if err != nil {
				g.showErr(err)
				return
			}
			refresh()
		})
	}()
}

// remoteRemoveMember drops a member from the party (by roster id when set, else
// by name) and saves the result.
func (g *gui) remoteRemoveMember(name string, target domain.Character, refresh func()) {
	go func() {
		ctx, cancel := bg(15)
		members, err := g.remote.Party(ctx, name)
		if err == nil {
			next := make([]*domain.Character, 0, len(members))
			dropped := false
			for i := range members {
				m := members[i]
				match := (target.ID != "" && m.ID == target.ID) || (target.ID == "" && m.Name == target.Name)
				if match && !dropped {
					dropped = true
					continue
				}
				next = append(next, &m)
			}
			err = g.remote.SetParty(ctx, name, next)
		}
		cancel()
		fyne.Do(func() {
			if err != nil {
				g.showErr(err)
				return
			}
			refresh()
		})
	}()
}

// errString is a tiny error wrapper for showErr on a plain message.
type errString string

func (e errString) Error() string { return string(e) }
