package main

import (
	"embed"

	"github.com/tiredbooy/Rum/backend/cmd/server"
	"github.com/wailsapp/wails/v2/pkg/application"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
)

//go:embed all:frontend/dist
var assets embed.FS

func main() {
	app := NewApp()

	opts := &options.App{
		Title:  "Rum",
		Width:  768,
		Height: 576,
		AssetServer: &assetserver.Options{
			Assets: assets,
		},
		BackgroundColour: &options.RGBA{R: 27, G: 38, B: 54, A: 1},
		OnStartup:        app.startup,
		Bind: []interface{}{
			app,
		},
	}

	wailsApp := application.NewWithOptions(opts)

	server.SetQuitFunc(wailsApp.Quit)

	go server.Start()

	err := wailsApp.Run()
	if err != nil {
		println("Error:", err.Error())
	}
}
