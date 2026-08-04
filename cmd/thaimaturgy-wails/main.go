package main

import (
	"embed"
	"log"

	"github.com/theburrowhub/thaimaturgy/internal/wailsapp"
	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
)

//go:embed all:frontend/dist
var assets embed.FS

func main() {
	app, err := wailsapp.New()
	if err != nil {
		log.Fatal(err)
	}

	err = wails.Run(&options.App{
		Title:  "thAImaturgy — DM Oracle",
		Width:  1440,
		Height: 900,
		AssetServer: &assetserver.Options{
			Assets: assets,
		},
		BackgroundColour: &options.RGBA{R: 13, G: 16, B: 23, A: 1},
		OnStartup:        app.Startup,
		Bind: []interface{}{
			app,
		},
	})
	if err != nil {
		log.Fatal(err)
	}
}
