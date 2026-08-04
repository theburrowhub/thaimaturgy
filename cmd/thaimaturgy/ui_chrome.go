package main

import (
	"image/color"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
)

var (
	chromeNight   = color.NRGBA{R: 0x0B, G: 0x12, B: 0x24, A: 0xFF}
	panelPaper    = color.NRGBA{R: 0xF6, G: 0xEF, B: 0xDE, A: 0xFF}
	panelPaperAlt = color.NRGBA{R: 0xEA, G: 0xDF, B: 0xC7, A: 0xFF}
	panelInk      = color.NRGBA{R: 0x12, G: 0x25, B: 0x4A, A: 0xFF}
	panelMuted    = color.NRGBA{R: 0x68, G: 0x5B, B: 0x48, A: 0xFF}
	panelGold     = color.NRGBA{R: 0xC9, G: 0x92, B: 0x2E, A: 0xFF}
)

func appShell(obj fyne.CanvasObject) fyne.CanvasObject {
	return container.NewStack(canvas.NewRectangle(chromeNight), container.NewPadded(obj))
}

func modernPanel(title, subtitle string, body fyne.CanvasObject) fyne.CanvasObject {
	bg := canvas.NewRectangle(panelPaper)
	stripe := canvas.NewRectangle(panelGold)
	stripe.SetMinSize(fyne.NewSize(4, 1))

	titleText := canvas.NewText(title, panelInk)
	titleText.TextStyle = fyne.TextStyle{Bold: true}
	titleText.TextSize = 17

	headerItems := []fyne.CanvasObject{titleText}
	if subtitle != "" {
		sub := canvas.NewText(subtitle, panelMuted)
		sub.TextSize = 11
		headerItems = append(headerItems, sub)
	}
	header := container.NewVBox(headerItems...)
	content := container.NewBorder(header, nil, stripe, nil, container.NewPadded(body))
	return container.NewStack(bg, container.NewPadded(content))
}

func modernToolbar(title string, actions ...fyne.CanvasObject) fyne.CanvasObject {
	bg := canvas.NewRectangle(panelPaper)
	kicker := canvas.NewText("DM ORACLE", panelGold)
	kicker.TextSize = 11
	kicker.TextStyle = fyne.TextStyle{Bold: true}
	titleText := canvas.NewText(title, panelInk)
	titleText.TextSize = 22
	titleText.TextStyle = fyne.TextStyle{Bold: true}
	brand := container.NewVBox(kicker, titleText)
	return container.NewStack(bg, container.NewPadded(container.NewBorder(nil, nil, brand, container.NewHBox(actions...), nil)))
}

func modernLibraryHero(title, subtitle string, actions fyne.CanvasObject) fyne.CanvasObject {
	bg := canvas.NewRectangle(chromeNight)
	kicker := canvas.NewText("THAIMATURGY", panelGold)
	kicker.TextSize = 12
	kicker.TextStyle = fyne.TextStyle{Bold: true}
	t := canvas.NewText(title, color.NRGBA{R: 0xF8, G: 0xF1, B: 0xDD, A: 0xFF})
	t.TextSize = 34
	t.TextStyle = fyne.TextStyle{Bold: true}
	s := canvas.NewText(subtitle, color.NRGBA{R: 0xD6, G: 0xC7, B: 0xA9, A: 0xFF})
	s.TextSize = 15
	copy := container.NewVBox(kicker, t, s)
	return container.NewStack(bg, container.NewPadded(container.NewBorder(nil, nil, copy, actions, nil)))
}
