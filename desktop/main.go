package main

import (
	"embed"
	"log"

	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
	"github.com/wailsapp/wails/v2/pkg/options/mac"
)

//go:embed all:frontend/dist
var assets embed.FS

func main() {
	app := NewApp()

	err := wails.Run(&options.App{
		Title:            "Haft",
		Width:            1280,
		Height:           800,
		MinWidth:         800,
		MinHeight:        600,
		DisableResize:    false,
		Frameless:        false,
		StartHidden:      false,
		BackgroundColour: &options.RGBA{R: 15, G: 15, B: 15, A: 255},
		AssetServer: &assetserver.Options{
			Assets: assets,
		},
		OnStartup:  app.startup,
		OnShutdown: app.shutdown,
		Bind: []interface{}{
			app,
		},
		Mac: &mac.Options{
			TitleBar:             mac.TitleBarHiddenInset(),
			WebviewIsTransparent: true,
			WindowIsTranslucent:  true,
			About: &mac.AboutInfo{
				Title:   "Haft",
				Message: "Engineering reasoning runtime",
			},
		},
	})
	if err != nil {
		log.Fatal(err)
	}
}
