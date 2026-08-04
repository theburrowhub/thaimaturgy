// Package guitheme provides the dark DM-cockpit Fyne theme shared by the
// desktop app and editor. It intentionally mirrors Tony's approved
// dm-oracle-moderno.html design: near-black chrome, muted dark panels, gold
// primary/read-aloud accents and arcane cyan focus/oracle states.
package guitheme

import (
	"image/color"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/theme"
)

// Palette lifted from .hermes/designs/dm-oracle-moderno-spec.md.
var (
	cBg         = rgb(0x0D, 0x10, 0x17) // --bg
	cPanel      = rgb(0x15, 0x19, 0x23) // --panel
	cPanel2     = rgb(0x1A, 0x1F, 0x2C) // --panel-2
	cText       = rgb(0xE9, 0xE9, 0xF0) // --text
	cMuted      = rgb(0x8B, 0x92, 0xA6) // --muted
	cFaint      = rgb(0x5D, 0x64, 0x78) // --faint
	cGold       = rgb(0xE2, 0xB2, 0x5C) // --gold
	cArcane     = rgb(0x7E, 0xE0, 0xD2) // --arcane
	cDanger     = rgb(0xE0, 0x70, 0x5C) // --danger
	cSelection  = rgba(0xE2, 0xB2, 0x5C, 0x30)
	cArcaneSoft = rgba(0x7E, 0xE0, 0xD2, 0x30)
	cLine       = rgba(0xFF, 0xFF, 0xFF, 0x13)
	cShadow     = rgba(0x00, 0x00, 0x00, 0xAA)
)

type cockpit struct{ base fyne.Theme }

// New returns the thAImaturgy desktop theme.
func New() fyne.Theme { return &cockpit{base: theme.DefaultTheme()} }

func (c *cockpit) Font(s fyne.TextStyle) fyne.Resource     { return c.base.Font(s) }
func (c *cockpit) Icon(n fyne.ThemeIconName) fyne.Resource { return c.base.Icon(n) }

func (c *cockpit) Size(n fyne.ThemeSizeName) float32 {
	switch n {
	case theme.SizeNameText:
		return 14
	case theme.SizeNamePadding:
		return 7
	case theme.SizeNameInnerPadding:
		return 10
	case theme.SizeNameHeadingText:
		return 22
	case theme.SizeNameSubHeadingText:
		return 16
	case theme.SizeNameInputRadius, theme.SizeNameSelectionRadius:
		return 8
	case theme.SizeNameInlineIcon:
		return 15
	}
	return c.base.Size(n)
}

func (c *cockpit) Color(n fyne.ThemeColorName, _ fyne.ThemeVariant) color.Color {
	switch n {
	case theme.ColorNameBackground:
		return cBg
	case theme.ColorNameForeground:
		return cText
	case theme.ColorNameForegroundOnPrimary:
		return cBg
	case theme.ColorNamePrimary, theme.ColorNameHyperlink:
		return cGold
	case theme.ColorNameButton:
		return cPanel2
	case theme.ColorNameInputBackground, theme.ColorNameMenuBackground, theme.ColorNameOverlayBackground:
		return cPanel
	case theme.ColorNameInputBorder, theme.ColorNameSeparator:
		return cLine
	case theme.ColorNameHover:
		return cPanel2
	case theme.ColorNamePressed, theme.ColorNameSelection:
		return cSelection
	case theme.ColorNameFocus:
		return cArcane
	case theme.ColorNamePlaceHolder, theme.ColorNameDisabled:
		return cFaint
	case theme.ColorNameError:
		return cDanger
	case theme.ColorNameSuccess:
		return cArcane
	case theme.ColorNameWarning:
		return cGold
	case theme.ColorNameScrollBar:
		return cFaint
	case theme.ColorNameShadow:
		return cShadow
	}
	return c.base.Color(n, theme.VariantDark)
}

func rgb(r, g, b uint8) color.NRGBA     { return color.NRGBA{R: r, G: g, B: b, A: 0xFF} }
func rgba(r, g, b, a uint8) color.NRGBA { return color.NRGBA{R: r, G: g, B: b, A: a} }
