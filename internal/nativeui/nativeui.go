// Package nativeui shows the operating system's own file/save/folder pickers
// and message dialogs, so the app uses native windows and controls for these
// interactions instead of drawing its own. It wraps ncruces/zenity (which uses
// the platform's native dialogs) and normalizes cancellation.
//
// These calls block until the user dismisses the dialog. When called from a
// GUI's UI goroutine, run them in a separate goroutine and marshal results back
// with the toolkit's thread-safe update mechanism.
package nativeui

import (
	"errors"

	"github.com/ncruces/zenity"
)

// Filter is a named set of filename glob patterns (e.g. {"Module", {"*.tar.gz"}}).
type Filter struct {
	Name     string
	Patterns []string
}

func filterOptions(filters []Filter) []zenity.Option {
	var opts []zenity.Option
	for _, f := range filters {
		opts = append(opts, zenity.FileFilter{Name: f.Name, Patterns: f.Patterns, CaseFold: true})
	}
	return opts
}

// OpenFile shows the native open-file dialog. ok is false if cancelled.
func OpenFile(title string, filters ...Filter) (path string, ok bool) {
	opts := append([]zenity.Option{zenity.Title(title)}, filterOptions(filters)...)
	p, err := zenity.SelectFile(opts...)
	return p, err == nil && p != ""
}

// OpenFolder shows the native folder-selection dialog.
func OpenFolder(title string) (path string, ok bool) {
	p, err := zenity.SelectFile(zenity.Title(title), zenity.Directory())
	return p, err == nil && p != ""
}

// SaveFile shows the native save-file dialog with a suggested filename.
func SaveFile(title, defaultName string, filters ...Filter) (path string, ok bool) {
	opts := append([]zenity.Option{
		zenity.Title(title),
		zenity.Filename(defaultName),
		zenity.ConfirmOverwrite(),
	}, filterOptions(filters)...)
	p, err := zenity.SelectFileSave(opts...)
	return p, err == nil && p != ""
}

// Info shows a native informational dialog.
func Info(title, message string) { _ = zenity.Info(message, zenity.Title(title)) }

// Error shows a native error dialog.
func Error(title, message string) { _ = zenity.Error(message, zenity.Title(title)) }

// Confirm shows a native yes/no dialog and reports whether the user confirmed.
func Confirm(title, message string) bool {
	err := zenity.Question(message, zenity.Title(title))
	return err == nil // nil = OK/Yes; ErrCanceled = No/Cancel
}

// Canceled reports whether an error from a dialog is a user cancellation.
func Canceled(err error) bool { return errors.Is(err, zenity.ErrCanceled) }
