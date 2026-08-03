package main

import (
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/driver/desktop"
	"fyne.io/fyne/v2/widget"
)

// chatEntry is a multi-line entry with chat-style Enter handling: plain Enter
// submits and Ctrl/Cmd+Enter inserts a newline (the opposite of Fyne's default).
//
// The GLFW driver routes a modifier+key combo through TypedShortcut (as a
// desktop.CustomShortcut) and a plain key through TypedKey, so the two cases are
// handled in those two methods respectively.
type chatEntry struct {
	widget.Entry
	onSubmit func(string)
}

func newChatEntry(onSubmit func(string)) *chatEntry {
	e := &chatEntry{onSubmit: onSubmit}
	e.MultiLine = true
	e.Wrapping = fyne.TextWrapWord
	e.ExtendBaseWidget(e)
	return e
}

// TypedKey submits on a plain Return/Enter (no modifier — a modifier+Enter is
// delivered to TypedShortcut instead, never here).
func (e *chatEntry) TypedKey(key *fyne.KeyEvent) {
	if key.Name == fyne.KeyReturn || key.Name == fyne.KeyEnter {
		if e.onSubmit != nil {
			e.onSubmit(e.Text)
		}
		return
	}
	e.Entry.TypedKey(key)
}

// TypedShortcut turns Ctrl/Cmd+Enter into a newline; every other shortcut (copy,
// cut, paste, select-all, …) passes through to the embedded Entry.
func (e *chatEntry) TypedShortcut(s fyne.Shortcut) {
	if cs, ok := s.(*desktop.CustomShortcut); ok &&
		(cs.KeyName == fyne.KeyReturn || cs.KeyName == fyne.KeyEnter) &&
		cs.Modifier&(fyne.KeyModifierControl|fyne.KeyModifierSuper) != 0 {
		e.Entry.TypedKey(&fyne.KeyEvent{Name: fyne.KeyReturn}) // default multi-line behaviour: insert newline
		return
	}
	e.Entry.TypedShortcut(s)
}
