package main

import (
	"fmt"
	"math/rand/v2"
	"sort"
	"strconv"
	"strings"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/widget"

	"github.com/theburrowhub/thaimaturgy/internal/domain"
)

// rollAbility4d6DropLowest rolls 4d6 and sums the highest three — the classic
// ability-score roll. Result is in [3, 18].
func rollAbility4d6DropLowest() int {
	d := []int{rand.IntN(6) + 1, rand.IntN(6) + 1, rand.IntN(6) + 1, rand.IntN(6) + 1}
	sort.Ints(d)
	return d[1] + d[2] + d[3] // drop the lowest
}

func clampScore(v int) int {
	if v < 1 {
		return 1
	}
	if v > 30 {
		return 30
	}
	return v
}

// showCharacterCreator opens a from-scratch player-character creator (issue #1):
// identity plus a choice of ability-score method — standard array, 4d6-drop-lowest
// rolls, or manual entry — then builds the sheet (racial bonuses + derived stats)
// and adds it to the party, optionally saving it to the roster.
func (g *gui) showCharacterCreator() {
	if g.session == nil {
		return
	}
	name := widget.NewEntry()
	name.SetPlaceHolder("Name")
	race := widget.NewSelectEntry(domain.Races)
	race.SetText("Human")
	class := widget.NewSelectEntry(domain.Classes)
	class.SetText("Fighter")
	level := widget.NewEntry()
	level.SetText("1")
	background := widget.NewEntry()
	background.SetText("Adventurer")
	alignment := widget.NewEntry()
	alignment.SetText("Neutral")

	// Six ability entries hold the BASE scores (before racial bonuses), in
	// STR,DEX,CON,INT,WIS,CHA order; the standard array seeds them.
	std := []int{15, 14, 13, 12, 10, 8}
	entries := map[domain.Ability]*widget.Entry{}
	grid := container.NewGridWithColumns(6)
	for i, ab := range abilityOrder {
		e := widget.NewEntry()
		e.SetText(strconv.Itoa(std[i]))
		entries[ab] = e
		grid.Add(container.NewVBox(
			widget.NewLabelWithStyle(ab.String(), fyne.TextAlignCenter, fyne.TextStyle{Bold: true}), e))
	}
	fillStd := widget.NewButton("Standard array", func() {
		for i, ab := range abilityOrder {
			entries[ab].SetText(strconv.Itoa(std[i]))
		}
	})
	fillRoll := widget.NewButton("Roll 4d6 (drop lowest)", func() {
		for _, ab := range abilityOrder {
			entries[ab].SetText(strconv.Itoa(rollAbility4d6DropLowest()))
		}
	})

	toRoster := widget.NewCheck("Also save to the roster", nil)
	status := widget.NewLabel("")
	status.Wrapping = fyne.TextWrapWord

	var pop *widget.PopUp
	create := widget.NewButtonWithIcon("Create", nil, func() {
		if name.Text == "" {
			status.SetText("Name is required.")
			return
		}
		var base domain.AbilityScores
		for _, ab := range abilityOrder {
			base.Set(ab, clampScore(atoiDefault(entries[ab].Text, 10)))
		}
		c := domain.GenerateCharacterWithAbilities(name.Text, race.Text, class.Text, atoiDefault(level.Text, 1), base)
		// Apply the Background/Alignment the form collected — the generator only sets
		// defaults, so without this the fields the user typed were silently dropped.
		// A blank field keeps the generator's default rather than storing "".
		if bg := strings.TrimSpace(background.Text); bg != "" {
			c.Background = bg
		}
		if al := strings.TrimSpace(alignment.Text); al != "" {
			c.Alignment = al
		}
		party := partyPointers(g.session.State.PartySnapshot())
		party = append(party, c)
		g.session.State.SetParty(party) // dedupes names
		if toRoster.Checked {
			if _, err := g.store.SaveCharacter(c); err != nil {
				status.SetText("Added to party, but roster save failed: " + err.Error())
			}
		}
		g.refreshPCPanel()
		g.autosave()
		pop.Hide()
	})
	cancel := widget.NewButton("Cancel", func() { pop.Hide() })

	form := widget.NewForm(
		widget.NewFormItem("Name", name),
		widget.NewFormItem("Race", race),
		widget.NewFormItem("Class", class),
		widget.NewFormItem("Level", level),
		widget.NewFormItem("Background", background),
		widget.NewFormItem("Alignment", alignment),
	)
	content := container.NewVBox(
		widget.NewLabelWithStyle("Create a character", fyne.TextAlignCenter, fyne.TextStyle{Bold: true}),
		widget.NewSeparator(),
		form,
		widget.NewLabelWithStyle("Ability scores (base, before racial bonuses)", fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
		container.NewHBox(fillStd, fillRoll),
		grid,
		widget.NewLabel(fmt.Sprintf("Racial bonuses for %s are added on top; HP/AC/etc. are derived.", race.Text)),
		toRoster,
		status,
		widget.NewSeparator(),
		container.NewHBox(create, cancel),
	)
	scroll := container.NewVScroll(container.NewPadded(content))
	scroll.SetMinSize(fyne.NewSize(460, 520))
	pop = widget.NewModalPopUp(scroll, g.win.Canvas())
	pop.Resize(fyne.NewSize(520, 600))
	pop.Show()
}
