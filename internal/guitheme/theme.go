// Package guitheme provides a warm, tabletop-RPG "parchment & ink" Fyne theme
// shared by the player GUI and the module editor, giving both a consistent,
// more professional D&D look regardless of the OS light/dark setting.
package guitheme

import (
	"image/color"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/theme"
)

// Palette — parchment background, dark ink text, burgundy/gold accents.
var (
	cParchment = rgb(0xF3, 0xE9, 0xCF) // window background
	cParchDark = rgb(0xE8, 0xDA, 0xB4) // panels / hover
	cInk       = rgb(0x2B, 0x20, 0x16) // primary text
	cInkSoft   = rgb(0x6B, 0x5A, 0x3E) // placeholder / secondary
	cBurgundy  = rgb(0x7B, 0x2D, 0x26) // primary accent (buttons, focus)
	cGold      = rgb(0xA9, 0x79, 0x1C) // headings / borders
	cInput     = rgb(0xFB, 0xF5, 0xE3) // entry background
	cButton    = rgb(0xE3, 0xD3, 0xAA) // button face
	cErr       = rgb(0x9E, 0x2B, 0x25)
	cOK        = rgb(0x4F, 0x7A, 0x3F)
	cOverlay   = rgb(0xF7, 0xEF, 0xD8)
	cSelection = rgba(0x7B, 0x2D, 0x26, 0x55)
	cFocus     = rgba(0xA9, 0x79, 0x1C, 0x99)
	cShadow    = rgba(0x2B, 0x20, 0x16, 0x33)
)

type parchment struct{ base fyne.Theme }

// New returns the parchment theme.
func New() fyne.Theme { return &parchment{base: theme.DefaultTheme()} }

func (p *parchment) Font(s fyne.TextStyle) fyne.Resource     { return p.base.Font(s) }
func (p *parchment) Icon(n fyne.ThemeIconName) fyne.Resource { return p.base.Icon(n) }

func (p *parchment) Size(n fyne.ThemeSizeName) float32 {
	switch n {
	case theme.SizeNameText:
		return 14
	case theme.SizeNamePadding:
		return 5
	case theme.SizeNameInnerPadding:
		return 8
	case theme.SizeNameHeadingText:
		return 22
	case theme.SizeNameSubHeadingText:
		return 17
	case theme.SizeNameInputRadius, theme.SizeNameSelectionRadius:
		return 5
	}
	return p.base.Size(n)
}

func (p *parchment) Color(n fyne.ThemeColorName, _ fyne.ThemeVariant) color.Color {
	switch n {
	case theme.ColorNameBackground:
		return cParchment
	case theme.ColorNameForeground, theme.ColorNameForegroundOnPrimary:
		if n == theme.ColorNameForegroundOnPrimary {
			return cParchment // light text on burgundy
		}
		return cInk
	case theme.ColorNamePrimary, theme.ColorNameHyperlink:
		return cBurgundy
	case theme.ColorNameButton:
		return cButton
	case theme.ColorNameInputBackground:
		return cInput
	case theme.ColorNameInputBorder, theme.ColorNameSeparator:
		return cGold
	case theme.ColorNameHover:
		return cParchDark
	case theme.ColorNamePressed:
		return cSelection
	case theme.ColorNameSelection:
		return cSelection
	case theme.ColorNameFocus:
		return cFocus
	case theme.ColorNamePlaceHolder:
		return cInkSoft
	case theme.ColorNameDisabled:
		return cInkSoft
	case theme.ColorNameError:
		return cErr
	case theme.ColorNameSuccess:
		return cOK
	case theme.ColorNameWarning:
		return cGold
	case theme.ColorNameScrollBar:
		return cInkSoft
	case theme.ColorNameShadow:
		return cShadow
	case theme.ColorNameMenuBackground, theme.ColorNameOverlayBackground:
		return cOverlay
	}
	return p.base.Color(n, theme.VariantLight)
}

func rgb(r, g, b uint8) color.NRGBA     { return color.NRGBA{R: r, G: g, B: b, A: 0xFF} }
func rgba(r, g, b, a uint8) color.NRGBA { return color.NRGBA{R: r, G: g, B: b, A: a} }
