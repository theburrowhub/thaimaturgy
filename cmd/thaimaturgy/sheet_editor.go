package main

import (
	"fmt"
	"strconv"
	"strings"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/widget"

	"github.com/theburrowhub/thaimaturgy/internal/domain"
)

// --- Pure parsing helpers (unit-tested without a running GUI) --------------

// atoiDefault parses an integer, returning def when the text isn't a valid int
// (so a stray character in a form field can't blow away a whole sheet).
func atoiDefault(s string, def int) int {
	if n, err := strconv.Atoi(strings.TrimSpace(s)); err == nil {
		return n
	}
	return def
}

// parseCSVList splits a comma/newline-separated list into trimmed, non-empty
// entries.
func parseCSVList(s string) []string {
	fields := strings.FieldsFunc(s, func(r rune) bool { return r == ',' || r == '\n' })
	out := make([]string, 0, len(fields))
	for _, f := range fields {
		if t := strings.TrimSpace(f); t != "" {
			out = append(out, t)
		}
	}
	return out
}

// parseInventoryLines reads one item per line as "Name xN [E]", where "xN" sets
// the quantity (default 1) and a trailing "[E]" marks it equipped. Lines that are
// blank are skipped; an unparsable quantity falls back to 1.
func parseInventoryLines(text string) []domain.InventoryItem {
	var out []domain.InventoryItem
	for _, ln := range strings.Split(text, "\n") {
		ln = strings.TrimSpace(ln)
		if ln == "" {
			continue
		}
		equipped := false
		if strings.HasSuffix(strings.ToUpper(ln), "[E]") {
			equipped = true
			ln = strings.TrimSpace(ln[:len(ln)-3])
		}
		qty := 1
		if i := strings.LastIndex(ln, " x"); i >= 0 {
			if n, err := strconv.Atoi(strings.TrimSpace(ln[i+2:])); err == nil && n > 0 {
				qty = n
				ln = strings.TrimSpace(ln[:i])
			}
		}
		if ln == "" {
			continue
		}
		out = append(out, domain.InventoryItem{Name: ln, Quantity: qty, Equipped: equipped})
	}
	return out
}

// formatInventoryLines renders inventory for the editor (inverse of parse).
func formatInventoryLines(items []domain.InventoryItem) string {
	lines := make([]string, 0, len(items))
	for _, it := range items {
		s := it.Name
		if it.Quantity > 1 {
			s += fmt.Sprintf(" x%d", it.Quantity)
		}
		if it.Equipped {
			s += " [E]"
		}
		lines = append(lines, s)
	}
	return strings.Join(lines, "\n")
}

// parseSpellLines reads one spell per line as "Name | level | prepared", where
// level is a number (0 or "cantrip"/"c" for a cantrip) and a third field starting
// with "p" marks it prepared. Only the name is required.
func parseSpellLines(text string) []domain.Spell {
	var out []domain.Spell
	for _, ln := range strings.Split(text, "\n") {
		ln = strings.TrimSpace(ln)
		if ln == "" {
			continue
		}
		parts := strings.Split(ln, "|")
		name := strings.TrimSpace(parts[0])
		if name == "" {
			continue
		}
		sp := domain.Spell{Name: name}
		if len(parts) > 1 {
			lvl := strings.ToLower(strings.TrimSpace(parts[1]))
			if lvl != "c" && lvl != "cantrip" {
				sp.Level = atoiDefault(lvl, 0)
			}
		}
		if len(parts) > 2 && strings.HasPrefix(strings.ToLower(strings.TrimSpace(parts[2])), "p") {
			sp.Prepared = true
		}
		out = append(out, sp)
	}
	return out
}

// formatSpellLines renders a spellbook for the editor (inverse of parse).
func formatSpellLines(spells []domain.Spell) string {
	lines := make([]string, 0, len(spells))
	for _, sp := range spells {
		lvl := strconv.Itoa(sp.Level)
		if sp.Level == 0 {
			lvl = "cantrip"
		}
		row := fmt.Sprintf("%s | %s", sp.Name, lvl)
		if sp.Prepared {
			row += " | prepared"
		}
		lines = append(lines, row)
	}
	return strings.Join(lines, "\n")
}

// parseSlotSpec reads "1:4, 2:3, 3:2" (spellLevel:maxSlots) into a SpellSlots max
// table. Invalid or out-of-range levels are ignored.
func parseSlotSpec(s string) domain.SpellSlots {
	var slots domain.SpellSlots
	for _, part := range strings.FieldsFunc(s, func(r rune) bool { return r == ',' || r == '\n' }) {
		kv := strings.SplitN(strings.TrimSpace(part), ":", 2)
		if len(kv) != 2 {
			continue
		}
		lvl := atoiDefault(kv[0], 0)
		n := atoiDefault(kv[1], 0)
		if lvl >= 1 && lvl <= 9 && n >= 0 {
			slots.Max[lvl-1] = n
		}
	}
	return slots
}

// formatSlotSpec renders a slots max table for the editor (inverse of parse).
func formatSlotSpec(slots domain.SpellSlots) string {
	var parts []string
	for lvl := 1; lvl <= 9; lvl++ {
		if m := slots.MaxAt(lvl); m > 0 {
			parts = append(parts, fmt.Sprintf("%d:%d", lvl, m))
		}
	}
	return strings.Join(parts, ", ")
}

var abilityOrder = []domain.Ability{domain.STR, domain.DEX, domain.CON, domain.INT, domain.WIS, domain.CHA}

// --- Editor dialog ---------------------------------------------------------

// showSheetEditor opens a full D&D 5e character-sheet editor for the named party
// member, letting the user hand-edit every field (identity, abilities, combat,
// saves, skills, inventory, spells, notes). On save the edited sheet is
// normalized to a self-consistent state, applied under the session lock, logged
// to the timeline for the DM, and persisted.
func (g *gui) showSheetEditor(name string) {
	if g.session == nil {
		return
	}
	// Work from a value copy so cancelling changes nothing.
	var base domain.Character
	found := false
	for _, c := range g.session.State.PartySnapshot() {
		if strings.EqualFold(c.Name, name) {
			base = c
			found = true
			break
		}
	}
	if !found {
		return
	}

	nameE := widget.NewEntry()
	nameE.SetText(base.Name)
	raceE := widget.NewSelectEntry(domain.Races)
	raceE.SetText(base.Race)
	classE := widget.NewSelectEntry(domain.Classes)
	classE.SetText(base.Class)
	levelE := widget.NewEntry()
	levelE.SetText(strconv.Itoa(base.Level))
	bgE := widget.NewEntry()
	bgE.SetText(base.Background)
	alignE := widget.NewEntry()
	alignE.SetText(base.Alignment)

	abilityEntries := map[domain.Ability]*widget.Entry{}
	abilityGrid := container.NewGridWithColumns(6)
	for _, ab := range abilityOrder {
		e := widget.NewEntry()
		e.SetText(strconv.Itoa(base.Abilities.Get(ab)))
		abilityEntries[ab] = e
		abilityGrid.Add(container.NewVBox(
			widget.NewLabelWithStyle(ab.String(), fyne.TextAlignCenter, fyne.TextStyle{Bold: true}), e))
	}

	maxHPE := widget.NewEntry()
	maxHPE.SetText(strconv.Itoa(base.MaxHP))
	curHPE := widget.NewEntry()
	curHPE.SetText(strconv.Itoa(base.CurrentHP))
	tempHPE := widget.NewEntry()
	tempHPE.SetText(strconv.Itoa(base.TempHP))
	acE := widget.NewEntry()
	acE.SetText(strconv.Itoa(base.AC))
	initE := widget.NewEntry()
	initE.SetText(strconv.Itoa(base.Initiative))
	speedE := widget.NewEntry()
	speedE.SetText(strconv.Itoa(base.Speed))
	profE := widget.NewEntry()
	profE.SetText(strconv.Itoa(base.ProficiencyBonus))
	hdUsedE := widget.NewEntry()
	hdUsedE.SetText(strconv.Itoa(base.HitDiceUsed))
	inspC := widget.NewCheck("Inspiration", nil)
	inspC.SetChecked(base.Inspiration)
	goldE := widget.NewEntry()
	goldE.SetText(strconv.Itoa(base.Gold))
	xpE := widget.NewEntry()
	xpE.SetText(strconv.Itoa(base.XP))

	// Saving-throw proficiencies.
	saveNames := make([]string, len(abilityOrder))
	for i, ab := range abilityOrder {
		saveNames[i] = ab.String()
	}
	saveGroup := widget.NewCheckGroup(saveNames, nil)
	var selSaves []string
	for _, ab := range abilityOrder {
		if base.SaveProficient(ab) {
			selSaves = append(selSaves, ab.String())
		}
	}
	saveGroup.SetSelected(selSaves)
	saveGroup.Horizontal = true

	// Skill proficiency / expertise.
	skillNames := make([]string, len(domain.DefaultSkills))
	for i, s := range domain.DefaultSkills {
		skillNames[i] = s.Name
	}
	profGroup := widget.NewCheckGroup(skillNames, nil)
	expGroup := widget.NewCheckGroup(skillNames, nil)
	var selProf, selExp []string
	for _, s := range base.Skills {
		if s.Proficient {
			selProf = append(selProf, s.Name)
		}
		if s.Expert {
			selExp = append(selExp, s.Name)
		}
	}
	profGroup.SetSelected(selProf)
	expGroup.SetSelected(selExp)

	langE := widget.NewEntry()
	langE.SetText(strings.Join(base.Languages, ", "))
	otherProfE := widget.NewEntry()
	otherProfE.SetText(strings.Join(base.Proficiencies, ", "))

	invE := widget.NewMultiLineEntry()
	invE.SetMinRowsVisible(3)
	invE.SetText(formatInventoryLines(base.Inventory))

	// Spellcasting.
	castChoices := []string{"(none)", "STR", "DEX", "CON", "INT", "WIS", "CHA"}
	castSel := widget.NewSelect(castChoices, nil)
	slotsE := widget.NewEntry()
	spellsE := widget.NewMultiLineEntry()
	spellsE.SetMinRowsVisible(3)
	if base.Spellcasting != nil {
		castSel.SetSelected(base.Spellcasting.Ability.String())
		slotsE.SetText(formatSlotSpec(base.Spellcasting.Slots))
		spellsE.SetText(formatSpellLines(base.Spellcasting.Spells))
	} else {
		castSel.SetSelected("(none)")
	}

	notesE := widget.NewMultiLineEntry()
	notesE.SetMinRowsVisible(3)
	notesE.SetText(base.Notes)

	status := widget.NewLabel("")

	form := widget.NewForm(
		widget.NewFormItem("Name", nameE),
		widget.NewFormItem("Race", raceE),
		widget.NewFormItem("Class", classE),
		widget.NewFormItem("Level", levelE),
		widget.NewFormItem("Background", bgE),
		widget.NewFormItem("Alignment", alignE),
	)

	combatForm := widget.NewForm(
		widget.NewFormItem("Max HP", maxHPE),
		widget.NewFormItem("Current HP", curHPE),
		widget.NewFormItem("Temp HP", tempHPE),
		widget.NewFormItem("AC", acE),
		widget.NewFormItem("Initiative", initE),
		widget.NewFormItem("Speed", speedE),
		widget.NewFormItem("Prof. bonus", profE),
		widget.NewFormItem("Hit dice used", hdUsedE),
		widget.NewFormItem("Gold", goldE),
		widget.NewFormItem("XP", xpE),
	)

	spellForm := widget.NewForm(
		widget.NewFormItem("Casting ability", castSel),
		widget.NewFormItem("Slots (lvl:max)", slotsE),
	)

	content := container.NewVBox(
		widget.NewLabelWithStyle("Edit character sheet", fyne.TextAlignCenter, fyne.TextStyle{Bold: true}),
		widget.NewSeparator(),
		form,
		sectionLabel("Abilities"), abilityGrid,
		sectionLabel("Combat & resources"), combatForm, inspC,
		sectionLabel("Saving-throw proficiencies"), saveGroup,
		sectionLabel("Skills — proficient"), profGroup,
		sectionLabel("Skills — expertise"), expGroup,
		sectionLabel("Languages (comma-separated)"), langE,
		sectionLabel("Other proficiencies (comma-separated)"), otherProfE,
		sectionLabel("Inventory (one per line: Name xN [E])"), invE,
		sectionLabel("Spellcasting"), spellForm,
		widget.NewLabel("Spells (one per line: Name | level | prepared)"), spellsE,
		sectionLabel("Notes"), notesE,
		status,
	)

	var pop *widget.PopUp
	save := widget.NewButtonWithIcon("Save", nil, func() {
		edited := base // start from the snapshot, override edited fields

		edited.Name = strings.TrimSpace(nameE.Text)
		if edited.Name == "" {
			status.SetText("Name is required.")
			return
		}
		edited.Race = strings.TrimSpace(raceE.Text)
		edited.Class = strings.TrimSpace(classE.Text)
		edited.Level = atoiDefault(levelE.Text, base.Level)
		edited.Background = strings.TrimSpace(bgE.Text)
		edited.Alignment = strings.TrimSpace(alignE.Text)

		for _, ab := range abilityOrder {
			edited.Abilities.Set(ab, atoiDefault(abilityEntries[ab].Text, base.Abilities.Get(ab)))
		}

		edited.MaxHP = atoiDefault(maxHPE.Text, base.MaxHP)
		edited.CurrentHP = atoiDefault(curHPE.Text, base.CurrentHP)
		edited.TempHP = atoiDefault(tempHPE.Text, base.TempHP)
		edited.AC = atoiDefault(acE.Text, base.AC)
		edited.Initiative = atoiDefault(initE.Text, base.Initiative)
		edited.Speed = atoiDefault(speedE.Text, base.Speed)
		edited.ProficiencyBonus = atoiDefault(profE.Text, base.ProficiencyBonus)
		edited.HitDiceUsed = atoiDefault(hdUsedE.Text, base.HitDiceUsed)
		edited.Inspiration = inspC.Checked
		edited.Gold = atoiDefault(goldE.Text, base.Gold)
		edited.XP = atoiDefault(xpE.Text, base.XP)

		// Saving throws.
		edited.SavingThrows = nil
		selectedSaves := map[string]bool{}
		for _, s := range saveGroup.Selected {
			selectedSaves[s] = true
		}
		for _, ab := range abilityOrder {
			if selectedSaves[ab.String()] {
				edited.SavingThrows = append(edited.SavingThrows, ab)
			}
		}

		// Skills: rebuild from the default list, applying proficient/expert sets so
		// ability mappings stay canonical.
		profSet := map[string]bool{}
		for _, s := range profGroup.Selected {
			profSet[s] = true
		}
		expSet := map[string]bool{}
		for _, s := range expGroup.Selected {
			expSet[s] = true
		}
		skills := make([]domain.Skill, len(domain.DefaultSkills))
		copy(skills, domain.DefaultSkills)
		for i := range skills {
			skills[i].Proficient = profSet[skills[i].Name] || expSet[skills[i].Name]
			skills[i].Expert = expSet[skills[i].Name]
		}
		edited.Skills = skills

		edited.Languages = parseCSVList(langE.Text)
		edited.Proficiencies = parseCSVList(otherProfE.Text)
		edited.Inventory = parseInventoryLines(invE.Text)
		edited.Notes = strings.TrimSpace(notesE.Text)

		// Spellcasting.
		if castSel.Selected == "" || castSel.Selected == "(none)" {
			edited.Spellcasting = nil
		} else {
			sc := &domain.Spellcasting{Ability: abilityFromString(castSel.Selected)}
			sc.Slots = parseSlotSpec(slotsE.Text)
			sc.Spells = parseSpellLines(spellsE.Text)
			mod := domain.Modifier(edited.Abilities.Get(sc.Ability))
			sc.SaveDC = 8 + edited.ProficiencyBonus + mod
			sc.AttackBonus = edited.ProficiencyBonus + mod
			edited.Spellcasting = sc
		}

		edited.Normalize()

		if _, ok := g.session.State.MutateCharacter(name, func(c *domain.Character) { *c = edited }); !ok {
			status.SetText("Could not find the character to update.")
			return
		}
		g.session.State.AddNote(fmt.Sprintf("Sheet edited: %s", edited.Name))
		g.refreshPCPanel()
		g.autosave()
		pop.Hide()
	})
	cancel := widget.NewButton("Cancel", func() { pop.Hide() })

	scroll := container.NewVScroll(container.NewPadded(content))
	scroll.SetMinSize(fyne.NewSize(480, 560))
	box := container.NewBorder(nil, container.NewHBox(save, cancel), nil, nil, scroll)

	pop = widget.NewModalPopUp(container.NewPadded(box), g.win.Canvas())
	pop.Resize(fyne.NewSize(560, 680))
	pop.Show()
}

// abilityFromString maps an ability abbreviation to its domain.Ability.
func abilityFromString(s string) domain.Ability {
	switch strings.ToUpper(strings.TrimSpace(s)) {
	case "STR":
		return domain.STR
	case "DEX":
		return domain.DEX
	case "CON":
		return domain.CON
	case "INT":
		return domain.INT
	case "WIS":
		return domain.WIS
	case "CHA":
		return domain.CHA
	}
	return domain.INT
}
