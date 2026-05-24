package main

import (
	"context"
	"embed"
	"log"

	"github.com/tiredbooy/Rum/backend/cmd/server"
	"github.com/wailsapp/wails/v2/pkg/application"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
	"github.com/wailsapp/wails/v2/pkg/runtime"
)

type ConfirmDialogConfig struct {
	Title        string
	Message      string
	ConfirmLabel string
	CancelLabel  string
}

var DefaultConfirmConfig = ConfirmDialogConfig{
	Title:        "Exit Rum",
	Message:      "Are you sure you want to quit?",
	ConfirmLabel: "Yes",
	CancelLabel:  "No",
}

//go:embed all:frontend/dist
var assets embed.FS

var wailsApp *application.Application
var isQuitting bool // guard to prevent recursion

func main() {
	app := NewApp()

	pubSetting, err := server.LoadSettings()
	if err != nil {
		log.Println("Error loading settings:", err)
		return
	}

	opts := &options.App{
		Title:            "Rum",
		Width:            1024,
		Height:           700,
		MinWidth:         800,
		MinHeight:        600,
		WindowStartState: options.Normal,
		BackgroundColour: &options.RGBA{R: 27, G: 38, B: 54, A: 1},
		OnStartup:        app.startup,
		OnBeforeClose: func(ctx context.Context) bool {
			if isQuitting {
				return false // already quitting, do nothing
			}

			if !pubSetting.ConfirmOnExit {
				// No confirmation needed – quit the app directly
				isQuitting = true
				runtime.Quit(ctx)
				return false
			}

			cfg := DefaultConfirmConfig
			result, err := runtime.MessageDialog(ctx, runtime.MessageDialogOptions{
				Type:    runtime.QuestionDialog,
				Title:   cfg.Title,
				Message: cfg.Message,
				Buttons: []string{cfg.ConfirmLabel, cfg.CancelLabel},
			})
			if err != nil {
				log.Println("Confirm dialog failed:", err)
				return false // on error, prevent close
			}

			if result == cfg.ConfirmLabel {
				// User confirmed exit
				isQuitting = true
				runtime.Quit(ctx)
				return false
			}
			// User clicked Cancel – do nothing
			return false
		},
		SingleInstanceLock: &options.SingleInstanceLock{
			UniqueId: "com.tiredbooy.rum",
			OnSecondInstanceLaunch: func(data options.SecondInstanceData) {
				if app.ctx != nil {
					runtime.WindowShow(app.ctx)
				}
			},
		},
		Bind: []interface{}{app},
		AssetServer: &assetserver.Options{
			Assets: assets,
		},
	}

	wailsApp = application.NewWithOptions(opts)
	server.SetQuitFunc(wailsApp.Quit)

	go server.Start()

	err = wailsApp.Run()
	if err != nil {
		println("Error:", err.Error())
	}
}
