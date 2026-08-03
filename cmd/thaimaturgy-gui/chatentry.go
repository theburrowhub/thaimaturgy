package main

import (
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/driver/desktop"
	"fyne.io/fyne/v2/widget"
)

// chatEntry is a multi-line entry with chat-style Enter handling: plain Enter
// submits, and Ctrl/Cmd+Enter inserts a newline (the opposite of Fyne's default
// multi-line behaviour). Modifier state is tracked from KeyDown/KeyUp because
// fyne.KeyEvent carries no modifier flags.
type chatEntry struct {
	widget.Entry
	onSubmit func(string)
	modDown  bool
}

func newChatEntry(onSubmit func(string)) *chatEntry {
	e := &chatEntry{onSubmit: onSubmit}
	e.MultiLine = true
	e.Wrapping = fyne.TextWrapWord
	e.ExtendBaseWidget(e)
	return e
}

func isNewlineModifier(name fyne.KeyName) bool {
	switch name {
	case desktop.KeyControlLeft, desktop.KeyControlRight, desktop.KeySuperLeft, desktop.KeySuperRight:
		return true
	}
	return false
}

func (e *chatEntry) KeyDown(key *fyne.KeyEvent) {
	if isNewlineModifier(key.Name) {
		e.modDown = true
	}
	e.Entry.KeyDown(key)
}

func (e *chatEntry) KeyUp(key *fyne.KeyEvent) {
	if isNewlineModifier(key.Name) {
		e.modDown = false
	}
	e.Entry.KeyUp(key)
}

// TypedKey submits on a plain Return/Enter and inserts a newline when a
// Ctrl/Cmd modifier is held.
func (e *chatEntry) TypedKey(key *fyne.KeyEvent) {
	if key.Name == fyne.KeyReturn || key.Name == fyne.KeyEnter {
		if e.modDown {
			e.Entry.TypedKey(key) // Ctrl/Cmd+Enter → newline (default behaviour)
			return
		}
		if e.onSubmit != nil {
			e.onSubmit(e.Text)
		}
		return
	}
	e.Entry.TypedKey(key)
}
