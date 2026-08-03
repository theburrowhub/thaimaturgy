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
	"os"
	"path/filepath"

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
		zenity.Filename(saveTarget(defaultName)),
		zenity.ConfirmOverwrite(),
	}, filterOptions(filters)...)
	p, err := zenity.SelectFileSave(opts...)
	return p, err == nil && p != ""
}

// saveTarget resolves the initial path for a save dialog. If defaultName has no
// directory, or its directory no longer exists, it falls back to a guaranteed
// system location (the user's Documents, else home) so the dialog never opens in
// a missing directory and errors out.
func saveTarget(defaultName string) string {
	dir, base := filepath.Split(defaultName)
	dir = filepath.Clean(dir)
	if base == "" {
		base = "untitled"
	}
	if defaultName == base || !dirExists(dir) {
		dir = defaultSaveDir()
	}
	return filepath.Join(dir, base)
}

func defaultSaveDir() string {
	home, err := os.UserHomeDir()
	if err != nil || home == "" {
		if wd, werr := os.Getwd(); werr == nil {
			return wd
		}
		return "."
	}
	if docs := filepath.Join(home, "Documents"); dirExists(docs) {
		return docs
	}
	return home
}

func dirExists(p string) bool {
	info, err := os.Stat(p)
	return err == nil && info.IsDir()
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

// Choice shows a native question with two labeled buttons plus Cancel. It returns
// 1 for the primary button (primaryLabel), 2 for the secondary (secondaryLabel),
// or 0 if the dialog was canceled or dismissed.
func Choice(title, message, primaryLabel, secondaryLabel string) int {
	err := zenity.Question(message,
		zenity.Title(title),
		zenity.OKLabel(primaryLabel),
		zenity.ExtraButton(secondaryLabel),
		zenity.CancelLabel("Cancel"),
	)
	switch {
	case err == nil:
		return 1
	case errors.Is(err, zenity.ErrExtraButton):
		return 2
	default:
		return 0
	}
}

// Canceled reports whether an error from a dialog is a user cancellation.
func Canceled(err error) bool { return errors.Is(err, zenity.ErrCanceled) }
