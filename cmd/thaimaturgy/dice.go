package main

import (
	"fmt"
	"strconv"
	"strings"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/widget"

	"github.com/theburrowhub/thaimaturgy/internal/appservice"
	"github.com/theburrowhub/thaimaturgy/internal/domain"
	"github.com/theburrowhub/thaimaturgy/internal/engine"
)

// standardDice are the quick-roll buttons offered by the dice mini-app.
var standardDice = []int{4, 6, 8, 10, 12, 20, 100}

// showDiceRoller opens the dice mini-app: quick buttons for the standard polyhedral
// dice, a quantity/modifier row, and a free-form notation entry. Every roll is
// shown in the dialog, echoed to the oracle transcript, and logged to the session
// timeline (so it persists and appears in the Session Log).
func (g *gui) showDiceRoller() {
	if g.session == nil {
		return
	}
	if !g.hasLegacyDND5E() {
		g.showErr(appservice.ErrDNDUtilitiesUnavailable)
		return
	}

	result := widget.NewRichTextFromMarkdown("_Roll some dice…_")
	result.Wrapping = fyne.TextWrapWord

	qty := widget.NewEntry()
	qty.SetText("1")
	mod := widget.NewEntry()
	mod.SetText("0")

	roll := func(notation string) {
		md := g.rollDiceNotation(notation)
		result.ParseMarkdown(md)
	}

	// Quick dice buttons (respect the quantity + modifier fields).
	var quick []fyne.CanvasObject
	for _, sides := range standardDice {
		s := sides
		quick = append(quick, widget.NewButton(fmt.Sprintf("d%d", s), func() {
			roll(composeNotation(qty.Text, s, mod.Text))
		}))
	}
	diceGrid := container.NewGridWithColumns(4, quick...)

	qtyRow := container.NewGridWithColumns(2,
		container.NewBorder(nil, nil, widget.NewLabel("Count"), nil, qty),
		container.NewBorder(nil, nil, widget.NewLabel("Mod"), nil, mod),
	)

	notation := widget.NewEntry()
	notation.SetPlaceHolder("e.g. 2d6+3, 4d6, 1d20-1")
	notation.OnSubmitted = func(s string) { roll(s) }
	rollBtn := widget.NewButton("Roll", func() { roll(notation.Text) })
	notationRow := container.NewBorder(nil, nil, nil, rollBtn, notation)

	var pop *widget.PopUp
	closeBtn := widget.NewButton("Close", func() { pop.Hide() })

	content := container.NewVBox(
		widget.NewLabelWithStyle("🎲 Dice", fyne.TextAlignCenter, fyne.TextStyle{Bold: true}),
		widget.NewSeparator(),
		widget.NewLabelWithStyle("Quick roll", fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
		qtyRow,
		diceGrid,
		widget.NewSeparator(),
		widget.NewLabelWithStyle("Custom notation", fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
		notationRow,
		widget.NewSeparator(),
		result,
		widget.NewSeparator(),
		closeBtn,
	)

	// Use a core-widget modal popup rather than the dialog package so the feature
	// pulls in no extra module dependency.
	pop = widget.NewModalPopUp(container.NewPadded(content), g.win.Canvas())
	pop.Resize(fyne.NewSize(380, 460))
	pop.Show()
}

// composeNotation builds a dice notation string from the quantity/sides/modifier
// fields, tolerating blank or malformed count/modifier inputs.
func composeNotation(qtyStr string, sides int, modStr string) string {
	n, err := strconv.Atoi(strings.TrimSpace(qtyStr))
	if err != nil || n < 1 {
		n = 1
	}
	notation := fmt.Sprintf("%dd%d", n, sides)
	if m, err := strconv.Atoi(strings.TrimSpace(modStr)); err == nil && m != 0 {
		if m > 0 {
			notation += fmt.Sprintf("+%d", m)
		} else {
			notation += strconv.Itoa(m)
		}
	}
	return notation
}

// rollDiceNotation rolls the given notation, logs it to the session timeline and
// the oracle transcript, and returns a Markdown summary for the dice dialog.
func (g *gui) rollDiceNotation(notation string) string {
	if !g.hasLegacyDND5E() {
		return "⚠ " + appservice.ErrDNDUtilitiesUnavailable.Error()
	}
	notation = strings.TrimSpace(notation)
	if notation == "" {
		return "_Enter a dice notation._"
	}
	dr, err := engine.RollDice(notation)
	if err != nil {
		return "⚠ " + err.Error()
	}
	msg := fmt.Sprintf("🎲 %s: %s", dr.String(), dr.ResultString())
	if dr.IsCriticalHit() {
		msg += "  **CRIT!**"
	} else if dr.IsCriticalFail() {
		msg += "  **FUMBLE!**"
	}
	// Persist to the session timeline (shows in the Session Log and journal).
	g.session.State.AppendLog(domain.LogEntry{Type: domain.LogRoll, Message: strings.ReplaceAll(msg, "**", "")})
	g.session.MarkModified()
	g.appendTranscript(msg)
	g.refreshLog()
	g.autosave()
	return fmt.Sprintf("### %d\n\n%s", dr.Total, msg)
}
