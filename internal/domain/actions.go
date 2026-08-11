package domain

import "strings"

// ActionsHeadings are the exact headings the virtual DM places, on its own final
// line, before the players' suggested next actions (see the GM prompt). A
// frontend can detect the heading to hide the actions list — e.g. behind a
// Telegram spoiler (#63) — so players aren't spoiled by options they haven't
// discovered. One heading per supported language.
var ActionsHeadings = []string{"Posibles acciones:", "Possible actions:"}

// SplitActions separates a DM narration into the narrative and its trailing
// suggested-actions list, using an ActionsHeadings marker at the start of a line.
// It returns the narrative (before the heading), the matched heading text, and
// the actions list (text after the heading). heading and actions are "" when no
// marker is present, in which case narration is the whole (trimmed) text.
func SplitActions(text string) (narration, heading, actions string) {
	best := -1
	for _, h := range ActionsHeadings {
		if i := lastHeadingLineIndex(text, h); i > best {
			best, heading = i, h
		}
	}
	if best < 0 {
		return strings.TrimRight(text, " \t\n"), "", ""
	}
	narration = strings.TrimRight(text[:best], " \t\n")
	actions = strings.TrimSpace(text[best+len(heading):])
	return narration, heading, actions
}

// lastHeadingLineIndex returns the byte index of the LAST occurrence of heading
// at the start of a line (case-insensitive; leading spaces/tabs allowed), or -1.
func lastHeadingLineIndex(text, heading string) int {
	lower := strings.ToLower(text)
	h := strings.ToLower(heading)
	best, from := -1, 0
	for {
		i := strings.Index(lower[from:], h)
		if i < 0 {
			break
		}
		abs := from + i
		if atLineStart(text, abs) {
			best = abs
		}
		from = abs + len(h)
	}
	return best
}

// atLineStart reports whether byte index i begins a line, allowing leading spaces
// or tabs after the newline.
func atLineStart(text string, i int) bool {
	for j := i - 1; j >= 0; j-- {
		switch text[j] {
		case '\n':
			return true
		case ' ', '\t':
			continue
		default:
			return false
		}
	}
	return true
}
