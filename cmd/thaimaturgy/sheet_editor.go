package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/widget"

	"github.com/theburrowhub/thaimaturgy/internal/domain"
)

// allConditions is the full set of 5e conditions, in canonical order, for the
// editor's condition selector.
var allConditions = []domain.Condition{
	domain.ConditionBlinded, domain.ConditionCharmed, domain.ConditionDeafened,
	domain.ConditionExhausted, domain.ConditionFrightened, domain.ConditionGrappled,
	domain.ConditionIncapacitated, domain.ConditionInvisible, domain.ConditionParalyzed,
	domain.ConditionPetrified, domain.ConditionPoisoned, domain.ConditionProne,
	domain.ConditionRestrained, domain.ConditionStunned, domain.ConditionUnconscious,
}

// Feature lines use one trait per physical line as "Name | Source | Description".
// Because a Description may itself contain '|' or newlines, each field is escaped
// (\\, \|, \n) so the format round-trips losslessly — a multi-line description
// survives an open/save cycle instead of being split into bogus extra traits.

func escapeField(s string) string {
	s = strings.ReplaceAll(s, `\`, `\\`)
	s = strings.ReplaceAll(s, "|", `\|`)
	s = strings.ReplaceAll(s, "\r\n", "\n")
	s = strings.ReplaceAll(s, "\n", `\n`)
	s = strings.ReplaceAll(s, "\r", `\n`)
	return s
}

func unescapeField(s string) string {
	var b strings.Builder
	for i := 0; i < len(s); i++ {
		if s[i] == '\\' && i+1 < len(s) {
			switch s[i+1] {
			case 'n':
				b.WriteByte('\n')
			case '\\':
				b.WriteByte('\\')
			case '|':
				b.WriteByte('|')
			default:
				b.WriteByte(s[i+1])
			}
			i++
			continue
		}
		b.WriteByte(s[i])
	}
	return b.String()
}

// splitEscapedPipe splits on unescaped '|' into at most n parts.
func splitEscapedPipe(s string, n int) []string {
	var parts []string
	var cur strings.Builder
	for i := 0; i < len(s); i++ {
		if s[i] == '\\' && i+1 < len(s) {
			cur.WriteByte(s[i])
			cur.WriteByte(s[i+1])
			i++
			continue
		}
		if s[i] == '|' && (n <= 0 || len(parts) < n-1) {
			parts = append(parts, cur.String())
			cur.Reset()
			continue
		}
		cur.WriteByte(s[i])
	}
	parts = append(parts, cur.String())
	return parts
}

// parseFeatureLines reads one trait per physical line as
// "Name | Source | Description", unescaping each field (only the name is
// required). Embedded newlines/pipes are preserved via the escaping scheme.
func parseFeatureLines(text string) []domain.Trait {
	var out []domain.Trait
	for _, ln := range strings.Split(text, "\n") {
		if strings.TrimSpace(ln) == "" {
			continue
		}
		parts := splitEscapedPipe(ln, 3)
		name := strings.TrimSpace(unescapeField(parts[0]))
		if name == "" {
			continue
		}
		t := domain.Trait{Name: name}
		if len(parts) > 1 {
			t.Source = strings.TrimSpace(unescapeField(parts[1]))
		}
		if len(parts) > 2 {
			t.Description = strings.TrimSpace(unescapeField(parts[2]))
		}
		out = append(out, t)
	}
	return out
}

// formatFeatureLines renders traits for the editor (inverse of parse), escaping
// each field so embedded pipes/newlines survive the round-trip.
func formatFeatureLines(traits []domain.Trait) string {
	lines := make([]string, 0, len(traits))
	for _, t := range traits {
		lines = append(lines, strings.Join([]string{
			escapeField(t.Name), escapeField(t.Source), escapeField(t.Description),
		}, " | "))
	}
	return strings.Join(lines, "\n")
}

// mergeSpellMetadata copies School/Description from the previous spellbook onto
// the edited spells (matched by name, case-insensitive), since the editor only
// exposes name/level/prepared. Without this, saving would erase that metadata.
func mergeSpellMetadata(edited, prev []domain.Spell) []domain.Spell {
	meta := make(map[string]domain.Spell, len(prev))
	for _, sp := range prev {
		meta[strings.ToLower(sp.Name)] = sp
	}
	for i := range edited {
		if old, ok := meta[strings.ToLower(edited[i].Name)]; ok {
			edited[i].School = old.School
			edited[i].Description = old.Description
		}
	}
	return edited
}

// mergeInventoryMetadata copies unexposed item fields (Weight) from the previous
// inventory onto freshly parsed items, matched by name (case-insensitive), since
// the editor's line format carries only name/quantity/equipped. Without this,
// opening and saving a sheet would erase every item's weight.
func mergeInventoryMetadata(edited, prev []domain.InventoryItem) []domain.InventoryItem {
	weights := make(map[string]float64, len(prev))
	for _, it := range prev {
		weights[strings.ToLower(it.Name)] = it.Weight
	}
	for i := range edited {
		if w, ok := weights[strings.ToLower(edited[i].Name)]; ok {
			edited[i].Weight = w
		}
	}
	return edited
}

// mergeSlotUsage preserves spent-slot counts from the previous spellcasting block
// onto a freshly parsed max table (clamped to the new maxima), so editing the
// slot maxima doesn't silently refill all spent slots.
func mergeSlotUsage(newSlots domain.SpellSlots, prev *domain.Spellcasting) domain.SpellSlots {
	if prev == nil {
		return newSlots
	}
	for i := range newSlots.Used {
		u := prev.Slots.Used[i]
		if u < 0 {
			u = 0
		}
		if u > newSlots.Max[i] {
			u = newSlots.Max[i]
		}
		newSlots.Used[i] = u
	}
	return newSlots
}

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
// table. It validates strictly: every non-empty entry must be "L:N" with L in
// 1..9 and N a non-negative integer, returning an error otherwise so a malformed
// entry (e.g. "2:x") is rejected instead of silently zeroing the slot.
func parseSlotSpec(s string) (domain.SpellSlots, error) {
	var slots domain.SpellSlots
	for _, part := range strings.FieldsFunc(s, func(r rune) bool { return r == ',' || r == '\n' }) {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		kv := strings.SplitN(part, ":", 2)
		if len(kv) != 2 {
			return slots, fmt.Errorf("bad slot entry %q (use level:count, e.g. 1:4)", part)
		}
		lvl, errL := strconv.Atoi(strings.TrimSpace(kv[0]))
		n, errN := strconv.Atoi(strings.TrimSpace(kv[1]))
		if errL != nil || errN != nil {
			return slots, fmt.Errorf("bad slot entry %q (use level:count, e.g. 1:4)", part)
		}
		if lvl < 1 || lvl > 9 {
			return slots, fmt.Errorf("slot level %d out of range (1-9)", lvl)
		}
		if n < 0 {
			return slots, fmt.Errorf("slot count for level %d must be ≥ 0", lvl)
		}
		slots.Max[lvl-1] = n
	}
	return slots, nil
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

// sheetEditorForm builds the full D&D 5e character-sheet editor UI for `base`
// (identity, abilities, combat, saves, skills, conditions, languages,
// inventory, features, spellcasting, notes) and returns the scrollable content,
// a status label for inline messages, and collect(): a function that reads the
// widgets into a normalized edited Character. collect reports ok=false and writes
// to the status label on a validation error (empty name, bad slot spec). The UI
// and field mapping are shared by the local editor (showSheetEditor) and the
// remote GUI editor (remoteEditSheet) so the two frontends stay at parity.
func sheetEditorForm(base domain.Character) (content fyne.CanvasObject, status *widget.Label, collect func() (domain.Character, bool)) {
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

	// Conditions.
	condNames := make([]string, len(allConditions))
	for i, c := range allConditions {
		condNames[i] = string(c)
	}
	condGroup := widget.NewCheckGroup(condNames, nil)
	var selConds []string
	for _, c := range base.Conditions {
		selConds = append(selConds, string(c))
	}
	condGroup.SetSelected(selConds)

	langE := widget.NewEntry()
	langE.SetText(strings.Join(base.Languages, ", "))
	otherProfE := widget.NewEntry()
	otherProfE.SetText(strings.Join(base.Proficiencies, ", "))

	featuresE := widget.NewMultiLineEntry()
	featuresE.SetMinRowsVisible(3)
	featuresE.SetText(formatFeatureLines(base.Features))

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

	status = widget.NewLabel("")
	status.Wrapping = fyne.TextWrapWord

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

	content = container.NewVBox(
		form,
		sectionLabel("Abilities"), abilityGrid,
		sectionLabel("Combat & resources"), combatForm, inspC,
		sectionLabel("Saving-throw proficiencies"), saveGroup,
		sectionLabel("Skills — proficient"), profGroup,
		sectionLabel("Skills — expertise"), expGroup,
		sectionLabel("Conditions"), condGroup,
		sectionLabel("Languages (comma-separated)"), langE,
		sectionLabel("Other proficiencies (comma-separated)"), otherProfE,
		sectionLabel("Inventory (one per line: Name xN [E])"), invE,
		sectionLabel("Features & traits (one per line: Name | Source | Description)"), featuresE,
		sectionLabel("Spellcasting"), spellForm,
		widget.NewLabel("Spells (one per line: Name | level | prepared)"), spellsE,
		sectionLabel("Notes"), notesE,
	)

	collect = func() (domain.Character, bool) {
		edited := base // start from the snapshot, override edited fields

		edited.Name = strings.TrimSpace(nameE.Text)
		if edited.Name == "" {
			status.SetText("Name is required.")
			return domain.Character{}, false
		}
		edited.Race = strings.TrimSpace(raceE.Text)
		edited.Class = strings.TrimSpace(classE.Text)
		edited.Background = strings.TrimSpace(bgE.Text)
		edited.Alignment = strings.TrimSpace(alignE.Text)

		// Every numeric field is parsed STRICTLY: a malformed value (e.g. "12x")
		// records the first offending field so collect() reports it and returns
		// false with the dialog kept open — never silently falling back to the
		// baseline and reporting a misleading partial "success".
		bad := ""
		getInt := func(label, text string) int {
			n, err := strconv.Atoi(strings.TrimSpace(text))
			if err != nil && bad == "" {
				bad = label
			}
			return n
		}
		edited.Level = getInt("Level", levelE.Text)
		for _, ab := range abilityOrder {
			edited.Abilities.Set(ab, getInt(ab.String(), abilityEntries[ab].Text))
		}
		edited.MaxHP = getInt("Max HP", maxHPE.Text)
		edited.CurrentHP = getInt("Current HP", curHPE.Text)
		edited.TempHP = getInt("Temp HP", tempHPE.Text)
		edited.AC = getInt("AC", acE.Text)
		edited.Initiative = getInt("Initiative", initE.Text)
		edited.Speed = getInt("Speed", speedE.Text)
		edited.ProficiencyBonus = getInt("Prof. bonus", profE.Text)
		edited.HitDiceUsed = getInt("Hit dice used", hdUsedE.Text)
		edited.Inspiration = inspC.Checked
		edited.Gold = getInt("Gold", goldE.Text)
		edited.XP = getInt("XP", xpE.Text)
		if bad != "" {
			status.SetText(bad + " must be a whole number.")
			return domain.Character{}, false
		}

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
		edited.Inventory = mergeInventoryMetadata(parseInventoryLines(invE.Text), base.Inventory)
		edited.Features = parseFeatureLines(featuresE.Text)
		edited.Notes = strings.TrimSpace(notesE.Text)

		// Conditions.
		edited.Conditions = nil
		for _, sel := range condGroup.Selected {
			edited.Conditions = append(edited.Conditions, domain.Condition(sel))
		}

		// Spellcasting. Preserve spent slots and per-spell metadata that the form
		// doesn't expose, so opening and saving an unchanged caster is a no-op.
		if castSel.Selected == "" || castSel.Selected == "(none)" {
			edited.Spellcasting = nil
		} else {
			maxSlots, err := parseSlotSpec(slotsE.Text)
			if err != nil {
				status.SetText("Slots: " + err.Error())
				return domain.Character{}, false
			}
			sc := &domain.Spellcasting{Ability: abilityFromString(castSel.Selected)}
			sc.Slots = mergeSlotUsage(maxSlots, base.Spellcasting)
			sc.Spells = mergeSpellMetadata(parseSpellLines(spellsE.Text), spellsOf(base.Spellcasting))
			mod := domain.Modifier(edited.Abilities.Get(sc.Ability))
			sc.SaveDC = 8 + edited.ProficiencyBonus + mod
			sc.AttackBonus = edited.ProficiencyBonus + mod
			edited.Spellcasting = sc
		}

		edited.Normalize()
		return edited, true
	}

	return content, status, collect
}

// showSheetEditor opens the full 5e character-sheet editor for the named local
// party member. On save the edited sheet is applied under the session lock — but
// only if the live record still matches the baseline captured when the dialog
// opened, so a concurrent write (the DM, a rest, a Telegram edit) is never
// clobbered — then logged to the timeline for the DM and persisted.
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
	// Baseline snapshot of the character as it was when the dialog opened.
	baseJSON, _ := json.Marshal(base)

	content, status, collect := sheetEditorForm(base)

	var pop *widget.PopUp
	save := widget.NewButtonWithIcon("Save", nil, func() {
		edited, ok := collect()
		if !ok {
			return // collect already reported the problem in status
		}
		// Reject a rename that collides (case-insensitive) with another member, so
		// characters stay addressable by name (the editor looks them up by name).
		if !strings.EqualFold(edited.Name, name) {
			for _, other := range g.session.State.PartyNames() {
				if strings.EqualFold(other, edited.Name) {
					status.SetText("Another character is already named " + edited.Name + ". Choose a different name.")
					return
				}
			}
		}
		// Apply under the lock, but only if the live record still matches the
		// baseline (otherwise a concurrent update would be clobbered — reject).
		conflict := false
		_, ok = g.session.State.MutateCharacter(name, func(c *domain.Character) {
			if cur, _ := json.Marshal(c); !bytes.Equal(cur, baseJSON) {
				conflict = true
				return
			}
			*c = edited
		})
		if !ok {
			status.SetText("Could not find the character to update.")
			return
		}
		if conflict {
			status.SetText("This sheet changed since you opened it (the DM or another update). Reopen “Edit sheet…” and re-apply your changes.")
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
	box := container.NewBorder(
		widget.NewLabelWithStyle("Edit character sheet", fyne.TextAlignCenter, fyne.TextStyle{Bold: true}),
		container.NewVBox(status, container.NewHBox(save, cancel)), nil, nil, scroll)

	pop = widget.NewModalPopUp(container.NewPadded(box), g.win.Canvas())
	pop.Resize(fyne.NewSize(560, 680))
	pop.Show()
}

// spellsOf returns a spellcasting block's spells (nil-safe).
func spellsOf(sc *domain.Spellcasting) []domain.Spell {
	if sc == nil {
		return nil
	}
	return sc.Spells
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
