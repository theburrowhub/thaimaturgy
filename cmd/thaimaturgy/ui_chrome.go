package main

import (
	"image/color"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/layout"

	"github.com/theburrowhub/thaimaturgy/internal/buildinfo"
)

// Desktop chrome tokens mirror .hermes/designs/dm-oracle-moderno.html.
var (
	chromeBg     = color.NRGBA{R: 0x0D, G: 0x10, B: 0x17, A: 0xFF}
	chromePanel  = color.NRGBA{R: 0x15, G: 0x19, B: 0x23, A: 0xFF}
	chromePanel2 = color.NRGBA{R: 0x1A, G: 0x1F, B: 0x2C, A: 0xFF}
	chromeText   = color.NRGBA{R: 0xE9, G: 0xE9, B: 0xF0, A: 0xFF}
	chromeMuted  = color.NRGBA{R: 0x8B, G: 0x92, B: 0xA6, A: 0xFF}
	chromeFaint  = color.NRGBA{R: 0x5D, G: 0x64, B: 0x78, A: 0xFF}
	chromeGold   = color.NRGBA{R: 0xE2, G: 0xB2, B: 0x5C, A: 0xFF}
	chromeArcane = color.NRGBA{R: 0x7E, G: 0xE0, B: 0xD2, A: 0xFF}
	chromeDanger = color.NRGBA{R: 0xE0, G: 0x70, B: 0x5C, A: 0xFF}
)

func appShell(obj fyne.CanvasObject) fyne.CanvasObject {
	// A small, faint version tag pinned to the bottom-right corner. canvas.Text is
	// not a widget, so it never intercepts clicks on the content beneath it.
	ver := canvas.NewText(buildinfo.String()+"  ", chromeFaint)
	ver.TextSize = 10
	corner := container.NewBorder(nil, container.NewHBox(layout.NewSpacer(), ver), nil, nil)
	return container.NewStack(canvas.NewRectangle(chromeBg), obj, corner)
}

func modernPanel(title, subtitle string, body fyne.CanvasObject) fyne.CanvasObject {
	bg := canvas.NewRectangle(chromePanel)
	divider := canvas.NewRectangle(color.NRGBA{R: 0xFF, G: 0xFF, B: 0xFF, A: 0x12})
	divider.SetMinSize(fyne.NewSize(1, 1))

	titleText := canvas.NewText(title, chromeText)
	titleText.TextStyle = fyne.TextStyle{Bold: true}
	titleText.TextSize = 14

	headerItems := []fyne.CanvasObject{titleText}
	if subtitle != "" {
		sub := canvas.NewText(subtitle, chromeFaint)
		sub.TextSize = 10
		headerItems = append(headerItems, sub)
	}
	header := container.NewPadded(container.NewBorder(nil, nil, container.NewVBox(headerItems...), nil, nil))
	content := container.NewBorder(container.NewVBox(header, divider), nil, nil, nil, container.NewPadded(body))
	return container.NewStack(bg, content)
}

func modernSessionToolbar(title string, crumb fyne.CanvasObject, actions ...fyne.CanvasObject) fyne.CanvasObject {
	bg := canvas.NewRectangle(chromeBg)
	line := canvas.NewRectangle(color.NRGBA{R: 0xFF, G: 0xFF, B: 0xFF, A: 0x12})
	line.SetMinSize(fyne.NewSize(1, 1))

	kicker := canvas.NewText("DM ORACLE", chromeGold)
	kicker.TextSize = 10
	kicker.TextStyle = fyne.TextStyle{Bold: true}
	titleText := canvas.NewText(title, chromeText)
	titleText.TextSize = 16
	titleText.TextStyle = fyne.TextStyle{Bold: true}
	brand := container.NewVBox(kicker, titleText)

	bar := container.NewPadded(container.NewBorder(nil, nil, brand, container.NewHBox(actions...), crumb))
	return container.NewBorder(nil, line, nil, nil, container.NewStack(bg, bar))
}

func modernToolbar(title string, actions ...fyne.CanvasObject) fyne.CanvasObject {
	return modernSessionToolbar(title, nil, actions...)
}

func modernLibraryHero(title, subtitle string, actions fyne.CanvasObject) fyne.CanvasObject {
	bg := canvas.NewRectangle(chromeBg)
	kicker := canvas.NewText("THAIMATURGY", chromeGold)
	kicker.TextSize = 11
	kicker.TextStyle = fyne.TextStyle{Bold: true}
	t := canvas.NewText(title, chromeText)
	t.TextSize = 30
	t.TextStyle = fyne.TextStyle{Bold: true}
	s := canvas.NewText(subtitle, chromeMuted)
	s.TextSize = 13
	copy := container.NewVBox(kicker, t, s)
	return container.NewStack(bg, container.NewPadded(container.NewBorder(nil, nil, copy, actions, nil)))
}
