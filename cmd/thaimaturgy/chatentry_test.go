package main

import (
	"testing"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/driver/desktop"
	"fyne.io/fyne/v2/test"
)

func TestChatEntryEnterSubmits(t *testing.T) {
	test.NewApp()

	var submitted string
	var calls int
	e := newChatEntry(func(s string) { submitted = s; calls++ })
	e.SetText("hello")
	e.TypedKey(&fyne.KeyEvent{Name: fyne.KeyReturn})
	if calls != 1 || submitted != "hello" {
		t.Fatalf("plain Enter should submit once with the text; calls=%d submitted=%q", calls, submitted)
	}

	// Disabled (busy): a keystroke must NOT submit, even though focus remains.
	calls = 0
	e.SetText("again")
	e.Disable()
	e.TypedKey(&fyne.KeyEvent{Name: fyne.KeyReturn})
	if calls != 0 {
		t.Errorf("disabled entry must not submit on Enter (calls=%d)", calls)
	}
	e.Enable()

	// Shift+Enter inserts a newline instead of submitting.
	calls = 0
	e.SetText("line1")
	e.KeyDown(&fyne.KeyEvent{Name: desktop.KeyShiftLeft})
	e.TypedKey(&fyne.KeyEvent{Name: fyne.KeyReturn})
	e.KeyUp(&fyne.KeyEvent{Name: desktop.KeyShiftLeft})
	if calls != 0 {
		t.Errorf("Shift+Enter must not submit (calls=%d)", calls)
	}
}

func TestChatEntryCtrlEnterNewline(t *testing.T) {
	test.NewApp()

	var calls int
	e := newChatEntry(func(string) { calls++ })
	e.SetText("hello")
	e.TypedShortcut(&desktop.CustomShortcut{KeyName: fyne.KeyReturn, Modifier: fyne.KeyModifierSuper})
	if calls != 0 {
		t.Errorf("Cmd+Enter must insert a newline, not submit (calls=%d)", calls)
	}
}
