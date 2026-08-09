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

// buildPCSheet renders a player character as a tabletop-style sheet: a header, a
// combat row and an ability grid of boxed stats, then gold/XP, conditions,
// inventory and notes. Returns the objects to place in the (scrolling) panel.
func buildPCSheet(c *domain.Character) []fyne.CanvasObject {
	objs := []fyne.CanvasObject{
		widget.NewLabelWithStyle(c.Name, fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
		widget.NewLabelWithStyle(pcSubtitle(c), fyne.TextAlignLeading, fyne.TextStyle{Italic: true}),
		widget.NewSeparator(),
	}

	// Combat row.
	hp := fmt.Sprintf("%d/%d", c.CurrentHP, c.MaxHP)
	if c.TempHP > 0 {
		hp += fmt.Sprintf(" +%d", c.TempHP)
	}
	objs = append(objs, container.NewGridWithColumns(4,
		statBox("HP", hp),
		statBox("AC", strconv.Itoa(c.AC)),
		statBox("Speed", strconv.Itoa(c.Speed)),
		statBox("Prof", fmt.Sprintf("+%d", c.ProficiencyBonus)),
	))

	// Ability grid (3×2).
	a := c.Abilities
	objs = append(objs, widget.NewSeparator(), container.NewGridWithColumns(3,
		abilityBox("STR", a.STR), abilityBox("DEX", a.DEX), abilityBox("CON", a.CON),
		abilityBox("INT", a.INT), abilityBox("WIS", a.WIS), abilityBox("CHA", a.CHA),
	))

	// Saving throws (proficient ones marked with •).
	saves := make([]string, 0, 6)
	for _, ab := range []domain.Ability{domain.STR, domain.DEX, domain.CON, domain.INT, domain.WIS, domain.CHA} {
		mark := ""
		if c.SaveProficient(ab) {
			mark = "•"
		}
		saves = append(saves, fmt.Sprintf("%s%s %+d", mark, ab, c.SaveBonus(ab)))
	}
	objs = append(objs, sectionLabel("Saving throws"), wrapLabel(strings.Join(saves, "   ")))

	line := fmt.Sprintf("Gold: %d      XP: %d      Hit dice: %d/%d", c.Gold, c.XP, c.HitDiceRemaining(), c.HitDiceMax())
	if c.Inspiration {
		line += "      ★ Inspiration"
	}
	objs = append(objs, widget.NewSeparator(), widget.NewLabel(line))

	if len(c.Languages) > 0 {
		objs = append(objs, sectionLabel("Languages"), wrapLabel(strings.Join(c.Languages, ", ")))
	}
	if len(c.Proficiencies) > 0 {
		objs = append(objs, sectionLabel("Proficiencies"), wrapLabel(strings.Join(c.Proficiencies, ", ")))
	}

	if len(c.Conditions) > 0 {
		conds := make([]string, len(c.Conditions))
		for i, cd := range c.Conditions {
			conds[i] = string(cd)
		}
		objs = append(objs, sectionLabel("Conditions"), wrapLabel(strings.Join(conds, ", ")))
	}

	if len(c.Inventory) > 0 {
		objs = append(objs, sectionLabel("Inventory"))
		for _, it := range c.Inventory {
			line := "• " + it.Name
			if it.Quantity > 1 {
				line += fmt.Sprintf(" ×%d", it.Quantity)
			}
			if it.Equipped {
				line += " (equipped)"
			}
			objs = append(objs, wrapLabel(line))
		}
	}

	if sc := c.Spellcasting; sc != nil {
		objs = append(objs, sectionLabel("Spellcasting"),
			wrapLabel(fmt.Sprintf("%s · Save DC %d · Attack %+d", sc.Ability, sc.SaveDC, sc.AttackBonus)))
		var slots []string
		for lvl := 1; lvl <= 9; lvl++ {
			if m := sc.Slots.MaxAt(lvl); m > 0 {
				slots = append(slots, fmt.Sprintf("L%d %d/%d", lvl, sc.Slots.RemainingAt(lvl), m))
			}
		}
		if len(slots) > 0 {
			objs = append(objs, wrapLabel("Slots: "+strings.Join(slots, "   ")))
		}
		for _, sp := range sc.Spells {
			lvl := "cantrip"
			if sp.Level > 0 {
				lvl = fmt.Sprintf("L%d", sp.Level)
			}
			row := fmt.Sprintf("• %s (%s)", sp.Name, lvl)
			if sp.Prepared {
				row += " — prepared"
			}
			objs = append(objs, wrapLabel(row))
		}
	}

	if len(c.Features) > 0 {
		objs = append(objs, sectionLabel("Features & traits"))
		for _, f := range c.Features {
			row := "• " + f.Name
			if f.Source != "" {
				row += " (" + f.Source + ")"
			}
			if f.Description != "" {
				row += ": " + f.Description
			}
			objs = append(objs, wrapLabel(row))
		}
	}

	if strings.TrimSpace(c.Notes) != "" {
		objs = append(objs, sectionLabel("Notes"), wrapLabel(c.Notes))
	}
	return objs
}

func pcSubtitle(c *domain.Character) string {
	sub := fmt.Sprintf("Level %d %s %s", c.Level, c.Race, c.Class)
	if c.Background != "" {
		sub += " · " + c.Background
	}
	if c.Alignment != "" {
		sub += " · " + c.Alignment
	}
	return sub
}

// abilityBox is a boxed ability score with its name, value and modifier.
func abilityBox(name string, score int) fyne.CanvasObject {
	return widget.NewCard("", "", container.NewVBox(
		widget.NewLabelWithStyle(name, fyne.TextAlignCenter, fyne.TextStyle{Bold: true}),
		widget.NewLabelWithStyle(strconv.Itoa(score), fyne.TextAlignCenter, fyne.TextStyle{}),
		widget.NewLabelWithStyle(domain.ModifierString(score), fyne.TextAlignCenter, fyne.TextStyle{Italic: true}),
	))
}

// statBox is a boxed labelled value (HP, AC, …).
func statBox(label, value string) fyne.CanvasObject {
	return widget.NewCard("", "", container.NewVBox(
		widget.NewLabelWithStyle(label, fyne.TextAlignCenter, fyne.TextStyle{Bold: true}),
		widget.NewLabelWithStyle(value, fyne.TextAlignCenter, fyne.TextStyle{}),
	))
}

func sectionLabel(title string) fyne.CanvasObject {
	return widget.NewLabelWithStyle(title, fyne.TextAlignLeading, fyne.TextStyle{Bold: true})
}

func wrapLabel(text string) fyne.CanvasObject {
	l := widget.NewLabel(text)
	l.Wrapping = fyne.TextWrapWord
	return l
}

// cleanMarkdown converts the lightweight markdown used in the transcript into
// plain text for the (selectable) chat entry: it drops bold/code markers, leading
// heading hashes and whole-line italic underscores, leaving readable prose.
func cleanMarkdown(s string) string {
	s = strings.ReplaceAll(s, "**", "")
	s = strings.ReplaceAll(s, "`", "")
	lines := strings.Split(s, "\n")
	for i, ln := range lines {
		// Strip a leading run of heading hashes and one following space.
		j := 0
		for j < len(ln) && ln[j] == '#' {
			j++
		}
		if j > 0 {
			ln = strings.TrimPrefix(ln[j:], " ")
		}
		// Unwrap a whole-line italic (_…_), used for status lines — but only when
		// the underscores are the single wrapping span (exactly two), so a line with
		// two separate italics isn't mangled. Preserve the original indentation.
		t := strings.TrimSpace(ln)
		if len(t) >= 2 && strings.HasPrefix(t, "_") && strings.HasSuffix(t, "_") && strings.Count(t, "_") == 2 {
			indent := ln[:len(ln)-len(strings.TrimLeft(ln, " \t"))]
			ln = indent + strings.TrimSuffix(strings.TrimPrefix(t, "_"), "_")
		}
		lines[i] = ln
	}
	return strings.Join(lines, "\n")
}
