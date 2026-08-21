package main

import (
	"context"
	"fmt"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"

	"github.com/theburrowhub/thaimaturgy/internal/domain"
)

// rosterOps abstracts the character-roster backend for the library-level
// character manager (#146), so one UI serves both the local disk store and a
// remote server. Every call may block (disk or network), so the manager invokes
// them off the UI thread and marshals results back via fyne.Do.
type rosterOps struct {
	subtitle string // header subtitle (source of the roster)
	back     func() // return to the owning library screen
	list     func(context.Context) ([]domain.Character, error)
	save     func(context.Context, *domain.Character) error
	remove   func(context.Context, string) error // by roster id
}

// localRosterOps drives the manager against the local on-disk roster (g.store).
func (g *gui) localRosterOps() rosterOps {
	return rosterOps{
		subtitle: "Local campaign roster",
		back:     g.showLibrary,
		list: func(context.Context) ([]domain.Character, error) {
			ptrs, err := g.store.ListCharacters()
			out := make([]domain.Character, 0, len(ptrs))
			for _, c := range ptrs {
				out = append(out, *c)
			}
			return out, err
		},
		save:   func(_ context.Context, c *domain.Character) error { _, err := g.store.SaveCharacter(c); return err },
		remove: func(_ context.Context, id string) error { return g.store.DeleteCharacter(id) },
	}
}

// remoteRosterOps drives the manager against the server's roster API.
func (g *gui) remoteRosterOps() rosterOps {
	return rosterOps{
		subtitle: "Roster on " + g.remote.BaseURL(),
		back:     g.showRemoteLibrary,
		list: func(ctx context.Context) ([]domain.Character, error) {
			ptrs, err := g.remote.ListCharacters(ctx)
			out := make([]domain.Character, 0, len(ptrs))
			for _, c := range ptrs {
				out = append(out, *c)
			}
			return out, err
		},
		save: func(ctx context.Context, c *domain.Character) error {
			_, err := g.remote.SaveCharacter(ctx, c)
			return err
		},
		remove: func(ctx context.Context, id string) error { return g.remote.DeleteCharacter(ctx, id) },
	}
}

// showRosterManager renders the library-level character manager: a list of the
// roster's characters with New / Edit / Delete, reachable from the main window
// without opening a session (#146). Editing reuses the shared full 5e sheet
// editor (sheetEditorForm).
func (g *gui) showRosterManager(ops rosterOps) {
	list := container.NewVBox()
	status := widget.NewLabel("")
	status.Wrapping = fyne.TextWrapWord

	var refresh func()
	refresh = func() {
		status.SetText("Loading…")
		go func() {
			ctx, cancel := bg(15)
			chars, err := ops.list(ctx)
			cancel()
			fyne.Do(func() {
				status.SetText("")
				objs := []fyne.CanvasObject{}
				if err != nil {
					objs = append(objs, wrapLabel("⚠ "+err.Error()))
				}
				if len(chars) == 0 && err == nil {
					objs = append(objs, widget.NewLabelWithStyle("The roster is empty.", fyne.TextAlignLeading, fyne.TextStyle{Italic: true}),
						wrapLabel("Use “New character” to add one."))
				}
				for i := range chars {
					c := chars[i] // stable copy for the row closures
					label := widget.NewLabel(fmt.Sprintf("%s — Lvl %d %s %s", c.Name, c.Level, c.Race, c.Class))
					edit := widget.NewButtonWithIcon("", theme.DocumentCreateIcon(), func() {
						g.editRosterCharacter(ops, c, status, refresh)
					})
					edit.Importance = widget.LowImportance
					del := widget.NewButtonWithIcon("", theme.DeleteIcon(), func() {
						g.deleteRosterCharacter(ops, c, status, refresh)
					})
					del.Importance = widget.LowImportance
					objs = append(objs, container.NewBorder(nil, nil, nil, container.NewHBox(edit, del), label))
				}
				list.Objects = objs
				list.Refresh()
			})
		}()
	}
	refresh()

	newBtn := widget.NewButtonWithIcon("New character", theme.ContentAddIcon(), func() {
		g.editRosterCharacter(ops, *domain.NewCharacter("New Hero", "Human", "Fighter"), status, refresh)
	})
	back := widget.NewButtonWithIcon("← Library", theme.NavigateBackIcon(), ops.back)

	header := container.NewVBox(
		container.NewHBox(back, layoutSpacer(), newBtn),
		widget.NewLabelWithStyle("Characters", fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
		widget.NewLabelWithStyle(ops.subtitle, fyne.TextAlignLeading, fyne.TextStyle{Italic: true}),
		widget.NewSeparator(),
	)
	content := container.NewBorder(header, status, nil, nil, container.NewVScroll(list))
	g.win.SetContent(appShell(container.NewPadded(content)))
}

// editRosterCharacter opens the shared sheet editor on a roster character (or a
// fresh one) and, on save, persists it via ops.save then refreshes the list.
func (g *gui) editRosterCharacter(ops rosterOps, base domain.Character, status *widget.Label, onDone func()) {
	content, formStatus, collect := sheetEditorForm(base)

	var pop *widget.PopUp
	var save *widget.Button
	save = widget.NewButtonWithIcon("Save", theme.DocumentSaveIcon(), func() {
		edited, ok := collect()
		if !ok {
			return // collect already reported the problem in formStatus
		}
		formStatus.SetText("")
		save.Disable()
		go func() {
			ctx, cancel := bg(20)
			err := ops.save(ctx, &edited)
			cancel()
			fyne.Do(func() {
				save.Enable()
				if err != nil {
					formStatus.SetText(err.Error()) // keep the dialog + values for a retry
					return
				}
				pop.Hide()
				status.SetText("Saved " + edited.Name + ".")
				onDone()
			})
		}()
	})
	cancel := widget.NewButton("Cancel", func() { pop.Hide() })

	scroll := container.NewVScroll(container.NewPadded(content))
	scroll.SetMinSize(fyne.NewSize(480, 560))
	box := container.NewBorder(
		widget.NewLabelWithStyle("Edit "+base.Name, fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
		container.NewVBox(formStatus, container.NewHBox(layoutSpacer(), cancel, save)), nil, nil, scroll)
	pop = widget.NewModalPopUp(container.NewPadded(box), g.win.Canvas())
	pop.Resize(fyne.NewSize(560, 680))
	pop.Show()
}

// deleteRosterCharacter removes a character from the roster (by id) then refreshes.
func (g *gui) deleteRosterCharacter(ops rosterOps, c domain.Character, status *widget.Label, onDone func()) {
	if c.ID == "" {
		status.SetText(c.Name + " has no roster id to delete.")
		return
	}
	go func() {
		ctx, cancel := bg(15)
		err := ops.remove(ctx, c.ID)
		cancel()
		fyne.Do(func() {
			if err != nil {
				status.SetText("⚠ " + err.Error())
				return
			}
			status.SetText("Deleted " + c.Name + " from the roster.")
			onDone()
		})
	}()
}
