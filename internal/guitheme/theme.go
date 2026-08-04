// Package guitheme provides a cinematic, modern tabletop-RPG Fyne theme
// shared by the player GUI and the module editor. It is inspired by dark
// starlit adventure pages: midnight chrome, warm parchment surfaces, sapphire
// focus states and restrained gold accents.
package guitheme

import (
	"image/color"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/theme"
)

// Palette — midnight shell, parchment work surfaces, sapphire/gold accents.
var (
	cNight     = rgb(0x0B, 0x12, 0x24) // app shell / gaps
	cNight2    = rgb(0x13, 0x22, 0x43) // hover chrome
	cPaper     = rgb(0xF6, 0xEF, 0xDE) // main panels
	cPaper2    = rgb(0xEA, 0xDF, 0xC7) // secondary panels / hover
	cPaper3    = rgb(0xFF, 0xFB, 0xF0) // inputs
	cInk       = rgb(0x18, 0x22, 0x32) // primary text
	cInkSoft   = rgb(0x68, 0x5B, 0x48) // placeholder / secondary
	cSapphire  = rgb(0x1D, 0x4E, 0x9B) // primary action
	cSapphire2 = rgb(0x2D, 0x6C, 0xD6) // focus / link
	cGold      = rgb(0xC9, 0x92, 0x2E) // accent / warnings
	cCopper    = rgb(0xB7, 0x4B, 0x3A) // danger / dramatic accent
	cOK        = rgb(0x3F, 0x7E, 0x53)
	cSelection = rgba(0x2D, 0x6C, 0xD6, 0x38)
	cFocus     = rgba(0xC9, 0x92, 0x2E, 0xAA)
	cShadow    = rgba(0x05, 0x08, 0x12, 0x88)
)

type parchment struct{ base fyne.Theme }

// New returns the thAImaturgy theme.
func New() fyne.Theme { return &parchment{base: theme.DefaultTheme()} }

func (p *parchment) Font(s fyne.TextStyle) fyne.Resource     { return p.base.Font(s) }
func (p *parchment) Icon(n fyne.ThemeIconName) fyne.Resource { return p.base.Icon(n) }

func (p *parchment) Size(n fyne.ThemeSizeName) float32 {
	switch n {
	case theme.SizeNameText:
		return 14
	case theme.SizeNamePadding:
		return 7
	case theme.SizeNameInnerPadding:
		return 10
	case theme.SizeNameHeadingText:
		return 24
	case theme.SizeNameSubHeadingText:
		return 17
	case theme.SizeNameInputRadius, theme.SizeNameSelectionRadius:
		return 9
	}
	return p.base.Size(n)
}

func (p *parchment) Color(n fyne.ThemeColorName, _ fyne.ThemeVariant) color.Color {
	switch n {
	case theme.ColorNameBackground:
		return cNight
	case theme.ColorNameForeground:
		return cInk
	case theme.ColorNameForegroundOnPrimary:
		return cPaper3
	case theme.ColorNamePrimary, theme.ColorNameHyperlink:
		return cSapphire
	case theme.ColorNameButton:
		return cPaper2
	case theme.ColorNameInputBackground, theme.ColorNameMenuBackground, theme.ColorNameOverlayBackground:
		return cPaper3
	case theme.ColorNameInputBorder, theme.ColorNameSeparator:
		return cGold
	case theme.ColorNameHover:
		return cPaper2
	case theme.ColorNamePressed, theme.ColorNameSelection:
		return cSelection
	case theme.ColorNameFocus:
		return cFocus
	case theme.ColorNamePlaceHolder, theme.ColorNameDisabled:
		return cInkSoft
	case theme.ColorNameError:
		return cCopper
	case theme.ColorNameSuccess:
		return cOK
	case theme.ColorNameWarning:
		return cGold
	case theme.ColorNameScrollBar:
		return cInkSoft
	case theme.ColorNameShadow:
		return cShadow
	}
	return p.base.Color(n, theme.VariantLight)
}

func rgb(r, g, b uint8) color.NRGBA     { return color.NRGBA{R: r, G: g, B: b, A: 0xFF} }
func rgba(r, g, b, a uint8) color.NRGBA { return color.NRGBA{R: r, G: g, B: b, A: a} }
