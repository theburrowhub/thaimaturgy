package main

import (
	"context"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"

	"github.com/theburrowhub/thaimaturgy/internal/domain"
)

// remotePartyPanel builds the editable party panel for the remote GUI (#130):
// the member sheets with a per-member Remove, plus a toolbar to add from the
// roster or set the default party. Every mutation goes through the HTTP API and
// then refetches so the panel reflects the server.
//
// Party edits are whole-party GET-modify-PUT, so overlapping edits could lose an
// update. All mutations here are therefore SERIALIZED behind a busy guard: while
// one is in flight the toolbar and Remove buttons are disabled, so this client
// can't interleave two edits. (Cross-client atomic/version-checked member ops are
// a tracked #130 follow-up.)
func (g *gui) remotePartyPanel(name string, initial *domain.SessionState) (fyne.CanvasObject, func()) {
	list := container.NewVBox()
	listScroll := container.NewVScroll(list)

	busy := false
	var addBtn, defBtn *widget.Button
	var removeBtns []*widget.Button // the per-member Remove buttons currently shown
	var refresh func()

	setBusy := func(b bool) {
		busy = b
		btns := append([]*widget.Button{addBtn, defBtn}, removeBtns...)
		for _, btn := range btns {
			if btn == nil {
				continue
			}
			if b {
				btn.Disable()
			} else {
				btn.Enable()
			}
		}
	}

	// mutate runs one party mutation serialized: it no-ops if another is already
	// in flight, disables the controls, runs fn (a blocking API call) off the UI
	// thread, then refetches. This prevents a double-click / overlapping edit from
	// losing an update via the GET-modify-PUT split.
	mutate := func(fn func(ctx context.Context) error) {
		if busy {
			return
		}
		setBusy(true)
		go func() {
			ctx, cancel := bg(20)
			err := fn(ctx)
			cancel()
			fyne.Do(func() {
				setBusy(false)
				if err != nil {
					g.showErr(err)
					return
				}
				refresh()
			})
		}()
	}

	// gen orders refreshes: a response is applied only if no newer refresh has
	// started since, so a slow request can't render a stale party last. gen is
	// touched only on the UI thread (refresh is always called from there).
	gen := 0
	refresh = func() {
		gen++
		myGen := gen
		go func() {
			ctx, cancel := bg(15)
			members, err := g.remote.Party(ctx, name)
			cancel()
			fyne.Do(func() {
				if myGen != gen {
					return // superseded by a newer refresh
				}
				if err != nil {
					g.showErr(err)
					return
				}
				removeBtns = g.fillRemoteParty(list, members, name, busy, mutate)
			})
		}()
	}

	if initial != nil {
		removeBtns = g.fillRemoteParty(list, initial.PartySnapshot(), name, busy, mutate)
	}

	addBtn = widget.NewButtonWithIcon("From roster…", theme.ContentAddIcon(), func() {
		g.remoteAddFromRoster(name, mutate)
	})
	defBtn = widget.NewButtonWithIcon("Default party", theme.AccountIcon(), func() {
		mutate(func(ctx context.Context) error { return g.remote.DefaultParty(ctx, name) })
	})
	toolbar := container.NewHBox(addBtn, defBtn)
	return container.NewBorder(toolbar, nil, nil, nil, listScroll), refresh
}

// fillRemoteParty renders the party members with a Remove button each and returns
// those buttons so the caller can disable them while a mutation is in flight.
// Remove goes through mutate so edits stay serialized.
func (g *gui) fillRemoteParty(list *fyne.Container, members []domain.Character, name string, busy bool, mutate func(func(context.Context) error)) []*widget.Button {
	list.Objects = nil
	if len(members) == 0 {
		list.Add(widget.NewLabelWithStyle("No party.", fyne.TextAlignLeading, fyne.TextStyle{Italic: true}))
		list.Refresh()
		return nil
	}
	removes := make([]*widget.Button, 0, len(members))
	for i := range members {
		m := members[i] // stable copy for the closure + &m in buildPCSheet
		remove := widget.NewButtonWithIcon("", theme.DeleteIcon(), func() {
			g.remoteRemoveMember(name, m, mutate)
		})
		remove.Importance = widget.LowImportance
		if busy {
			remove.Disable()
		}
		removes = append(removes, remove)
		title := widget.NewLabelWithStyle(m.Name, fyne.TextAlignLeading, fyne.TextStyle{Bold: true})
		list.Add(container.NewBorder(nil, nil, nil, remove, title))
		list.Objects = append(list.Objects, buildPCSheet(&m)...)
	}
	list.Refresh()
	return removes
}

// remoteAddFromRoster lists roster characters and adds the chosen one to the
// party (skipping one already present by id, mirroring the web dedup guard and
// the server-side SetParty guard).
func (g *gui) remoteAddFromRoster(name string, mutate func(func(context.Context) error)) {
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
					mutate(func(ctx context.Context) error { return g.appendPartyMember(ctx, name, rc) })
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

// appendPartyMember re-reads the party and PUTs it with one member appended.
func (g *gui) appendPartyMember(ctx context.Context, name string, add *domain.Character) error {
	members, err := g.remote.Party(ctx, name)
	if err != nil {
		return err
	}
	next := make([]*domain.Character, 0, len(members)+1)
	for i := range members {
		m := members[i]
		next = append(next, &m)
	}
	next = append(next, add)
	return g.remote.SetParty(ctx, name, next)
}

// remoteRemoveMember drops a member (by roster id when set, else by name) and
// PUTs the result.
func (g *gui) remoteRemoveMember(name string, target domain.Character, mutate func(func(context.Context) error)) {
	mutate(func(ctx context.Context) error {
		members, err := g.remote.Party(ctx, name)
		if err != nil {
			return err
		}
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
		return g.remote.SetParty(ctx, name, next)
	})
}

// errString is a tiny error wrapper for showErr on a plain message.
type errString string

func (e errString) Error() string { return string(e) }
