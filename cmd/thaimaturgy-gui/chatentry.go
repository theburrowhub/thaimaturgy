package main

import (
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/driver/desktop"
	"fyne.io/fyne/v2/widget"
)

// chatEntry is a multi-line entry with chat-style Enter handling: plain Enter
// submits, while Shift+Enter and Ctrl/Cmd+Enter insert a newline (the opposite
// of Fyne's default).
//
// The GLFW driver routes a Control/Super+key combo through TypedShortcut (as a
// desktop.CustomShortcut) and everything else through TypedKey; Shift+key stays
// in TypedKey, so shift state is tracked from KeyDown/KeyUp (KeyEvent carries no
// modifier flags).
type chatEntry struct {
	widget.Entry
	onSubmit  func(string)
	shiftDown bool
}

func newChatEntry(onSubmit func(string)) *chatEntry {
	e := &chatEntry{onSubmit: onSubmit}
	e.MultiLine = true
	e.Wrapping = fyne.TextWrapWord
	e.ExtendBaseWidget(e)
	return e
}

func isShiftKey(name fyne.KeyName) bool {
	return name == desktop.KeyShiftLeft || name == desktop.KeyShiftRight
}

func (e *chatEntry) KeyDown(key *fyne.KeyEvent) {
	if isShiftKey(key.Name) {
		e.shiftDown = true
	}
	e.Entry.KeyDown(key)
}

func (e *chatEntry) KeyUp(key *fyne.KeyEvent) {
	if isShiftKey(key.Name) {
		e.shiftDown = false
	}
	e.Entry.KeyUp(key)
}

// TypedKey submits on a plain Return/Enter and inserts a newline on Shift+Enter.
// When the entry is disabled (e.g. an oracle request is in flight) it delegates
// to the base widget so a keystroke can't bypass the busy guard and fire another
// submission — disabling a widget doesn't remove keyboard focus in Fyne.
func (e *chatEntry) TypedKey(key *fyne.KeyEvent) {
	if e.Disabled() {
		e.Entry.TypedKey(key)
		return
	}
	if key.Name == fyne.KeyReturn || key.Name == fyne.KeyEnter {
		if e.shiftDown {
			e.Entry.TypedKey(key) // Shift+Enter → newline
			return
		}
		if e.onSubmit != nil {
			e.onSubmit(e.Text)
		}
		return
	}
	e.Entry.TypedKey(key)
}

// TypedShortcut turns Ctrl/Cmd+Enter into a newline; every other shortcut (copy,
// cut, paste, select-all, …) passes through. Disabled entries delegate untouched.
func (e *chatEntry) TypedShortcut(s fyne.Shortcut) {
	if !e.Disabled() {
		if cs, ok := s.(*desktop.CustomShortcut); ok &&
			(cs.KeyName == fyne.KeyReturn || cs.KeyName == fyne.KeyEnter) &&
			cs.Modifier&(fyne.KeyModifierControl|fyne.KeyModifierSuper) != 0 {
			e.Entry.TypedKey(&fyne.KeyEvent{Name: fyne.KeyReturn}) // insert newline
			return
		}
	}
	e.Entry.TypedShortcut(s)
}
