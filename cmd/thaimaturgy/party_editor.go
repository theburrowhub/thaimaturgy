package main

import (
	"context"
	"fmt"
	"strings"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/widget"

	"github.com/theburrowhub/thaimaturgy/internal/domain"
	"github.com/theburrowhub/thaimaturgy/internal/engine"
)

// showPartyEditor opens a dialog to create or adjust the player party. It shows
// the current roster, offers a one-click default heterogeneous party, and lets
// the user shape it with a natural-language prompt driven by the LLM (e.g. "un
// enano, un elfo y un humano; el elfo mago; empiezan a nivel 3"). The AI only
// decides the roster; stats are generated from D&D rules.
func (g *gui) showPartyEditor() {
	if g.session == nil {
		return
	}

	summary := widget.NewLabel("")
	summary.Wrapping = fyne.TextWrapWord
	refreshSummary := func() {
		party := g.session.State.PartySnapshot()
		if len(party) == 0 {
			summary.SetText("(no party yet)")
			return
		}
		lines := make([]string, 0, len(party))
		for i := range party {
			c := party[i]
			lines = append(lines, fmt.Sprintf("• %s — Level %d %s %s", c.Name, c.Level, c.Race, c.Class))
		}
		summary.SetText(strings.Join(lines, "\n"))
	}
	refreshSummary()

	prompt := widget.NewMultiLineEntry()
	prompt.Wrapping = fyne.TextWrapWord
	prompt.SetMinRowsVisible(3)
	prompt.SetPlaceHolder("Describe or adjust the party in natural language…\ne.g. \"un enano, un elfo y un humano; el elfo o el humano ha de ser mago; empiezan a nivel 3\"")

	status := widget.NewLabel("")
	status.Wrapping = fyne.TextWrapWord

	var pop *widget.PopUp
	var aiBtn, defBtn, closeBtn *widget.Button

	setBusy := func(busy bool) {
		for _, b := range []*widget.Button{aiBtn, defBtn, closeBtn} {
			if b == nil {
				continue
			}
			if busy {
				b.Disable()
			} else {
				b.Enable()
			}
		}
	}

	apply := func(party []*domain.Character) {
		g.session.State.SetParty(party)
		refreshSummary()
		g.refreshPCPanel()
		g.autosave()
	}

	aiBtn = widget.NewButton("Generate with AI", func() {
		req := strings.TrimSpace(prompt.Text)
		if req == "" {
			status.SetText("Type a request first.")
			return
		}
		if g.prov == nil {
			status.SetText("No AI provider configured; use “Default party” or set an API key.")
			return
		}
		setBusy(true)
		status.SetText("Generating party…")
		current := g.session.State.PartySnapshot()
		model := g.config.Model
		go func() {
			party, err := engine.PlanParty(context.Background(), g.prov, model, req, current)
			fyne.Do(func() {
				setBusy(false)
				if err != nil {
					status.SetText("⚠ " + err.Error())
					return
				}
				apply(party)
				status.SetText(fmt.Sprintf("Party updated (%d members).", len(party)))
			})
		}()
	})

	defBtn = widget.NewButton("Default party", func() {
		apply(domain.DefaultParty())
		status.SetText("Default heterogeneous level-1 party created.")
	})

	rosterBtn := widget.NewButton("Roster…", func() {
		pop.Hide()
		g.showRoster()
	})

	closeBtn = widget.NewButton("Close", func() { pop.Hide() })

	content := container.NewVBox(
		widget.NewLabelWithStyle("Party", fyne.TextAlignCenter, fyne.TextStyle{Bold: true}),
		widget.NewSeparator(),
		widget.NewLabelWithStyle("Current roster", fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
		summary,
		widget.NewSeparator(),
		widget.NewLabelWithStyle("Adjust with AI", fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
		prompt,
		container.NewHBox(aiBtn, defBtn, rosterBtn),
		status,
		widget.NewSeparator(),
		closeBtn,
	)

	pop = widget.NewModalPopUp(container.NewPadded(content), g.win.Canvas())
	pop.Resize(fyne.NewSize(460, 520))
	pop.Show()
}
