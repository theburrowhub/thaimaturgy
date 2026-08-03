package engine

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/theburrowhub/thaimaturgy/internal/domain"
)

// RollTable rolls the table's dice (or, if none is given, a die sized to the row
// count) and returns the rolled value and the matched row. The row is nil if
// nothing matched. Rows match by their Roll range ("1", "1-3", "18-20", "01-05");
// if no row declares a range, the nth row (1-based) is used.
func RollTable(t *domain.Table) (int, *domain.TableRow) {
	if t == nil || len(t.Rows) == 0 {
		return 0, nil
	}
	dice := strings.TrimSpace(t.Dice)
	if dice == "" {
		dice = fmt.Sprintf("d%d", len(t.Rows))
	}
	roll := 0
	if dr, err := RollDice(dice); err == nil {
		roll = dr.Total
	} else if dr, err := RollDice(fmt.Sprintf("d%d", len(t.Rows))); err == nil {
		roll = dr.Total
	} else {
		roll = 1
	}
	for i := range t.Rows {
		if lo, hi, ok := parseRollRange(t.Rows[i].Roll); ok && roll >= lo && roll <= hi {
			return roll, &t.Rows[i]
		}
	}
	if roll >= 1 && roll <= len(t.Rows) {
		return roll, &t.Rows[roll-1]
	}
	return roll, nil
}

// RowText joins a row's cells into a single readable line.
func RowText(r *domain.TableRow) string {
	if r == nil {
		return ""
	}
	var cells []string
	for _, c := range r.Cells {
		if strings.TrimSpace(c) != "" {
			cells = append(cells, strings.TrimSpace(c))
		}
	}
	return strings.Join(cells, " · ")
}

func parseRollRange(s string) (lo, hi int, ok bool) {
	s = strings.TrimSpace(s)
	if s == "" {
		return 0, 0, false
	}
	if i := strings.IndexAny(s, "-–—"); i > 0 {
		a, e1 := rollNum(s[:i])
		b, e2 := rollNum(s[i+len("-"):])
		if e1 != nil || e2 != nil {
			return 0, 0, false
		}
		if a > b {
			a, b = b, a
		}
		return a, b, true
	}
	n, err := rollNum(s)
	if err != nil {
		return 0, 0, false
	}
	return n, n, true
}

func nameOrID(name, id string) string {
	if strings.TrimSpace(name) != "" {
		return name
	}
	return id
}

func rollNum(s string) (int, error) {
	n, err := strconv.Atoi(strings.TrimSpace(s))
	if err == nil && n == 0 {
		n = 100 // d100 tables use "00" for 100
	}
	return n, err
}

// TableMarkdown renders a table as a readable Markdown list, which displays well
// both in the GUI's rich text and the PDF exporter (neither renders Markdown
// grid tables). Rollable rows are shown as "**range** — result".
func TableMarkdown(t *domain.Table) string {
	if t == nil {
		return ""
	}
	var sb strings.Builder
	if d := strings.TrimSpace(t.Description); d != "" {
		sb.WriteString(d + "\n\n")
	}
	if d := strings.TrimSpace(t.Dice); d != "" {
		fmt.Fprintf(&sb, "**Roll:** %s\n\n", d)
	}
	if len(t.Headers) > 0 {
		fmt.Fprintf(&sb, "*Columns: %s*\n\n", strings.Join(t.Headers, " · "))
	}
	for i := range t.Rows {
		r := &t.Rows[i]
		result := RowText(r)
		if roll := strings.TrimSpace(r.Roll); roll != "" {
			fmt.Fprintf(&sb, "- **%s** — %s\n", roll, result)
		} else {
			fmt.Fprintf(&sb, "- %s\n", result)
		}
	}
	return strings.TrimSpace(sb.String())
}
