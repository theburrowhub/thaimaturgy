package main

import (
	"testing"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/test"
	"fyne.io/fyne/v2/widget"

	"github.com/theburrowhub/thaimaturgy/internal/domain"
)

// TestStartModeSelector verifies the library's "Start as" control drives the
// gui.startMode used when a new game is started (#143).
func TestStartModeSelector(t *testing.T) {
	test.NewApp()

	g := &gui{}
	sel := findSelect(g.startModeSelector())
	if sel == nil {
		t.Fatal("startModeSelector produced no *widget.Select")
	}
	// Defaults to Oracle.
	if sel.Selected != "Oracle" {
		t.Errorf("default selection = %q; want Oracle", sel.Selected)
	}
	sel.SetSelected("Virtual DM")
	if g.startMode != domain.ModeVirtualDM {
		t.Errorf("after selecting Virtual DM, startMode = %q; want %q", g.startMode, domain.ModeVirtualDM)
	}
	sel.SetSelected("Oracle")
	if g.startMode != domain.ModeAssistant {
		t.Errorf("after selecting Oracle, startMode = %q; want %q", g.startMode, domain.ModeAssistant)
	}
}

// findSelect walks a widget tree and returns the first *widget.Select found.
func findSelect(obj fyne.CanvasObject) *widget.Select {
	switch w := obj.(type) {
	case *widget.Select:
		return w
	case *fyne.Container:
		for _, c := range w.Objects {
			if s := findSelect(c); s != nil {
				return s
			}
		}
	}
	return nil
}
