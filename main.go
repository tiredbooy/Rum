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

	// Touch the engine's settings accessor early so a corrupt/missing settings
	// file is repaired (it rewrites defaults) before the UI loads. The desktop
	// layer reads the tray/window/autostart prefs from settings.json directly
	// (see settings.go) because PublicSettings only exposes ConfirmOnExit.
	if _, err := server.LoadSettings(); err != nil {
		log.Println("Error loading settings:", err)
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
		OnShutdown:       app.shutdown,
		OnBeforeClose: func(ctx context.Context) bool {
			if isQuitting {
				return false // already quitting, do nothing
			}

			// Re-read the desktop settings on every close attempt so a runtime
			// toggle of close-to-tray is honored without a restart. (server.LoadSettings
			// only exposes ConfirmOnExit; the tray/window prefs come from settings.json
			// directly — see settings.go.)
			ds := loadDesktopSettings()

			// Persist the current window geometry before any hide/quit so the next
			// launch restores it.
			persistWindowState(ctx, ds)

			// Close-to-tray: hide instead of quitting, and cancel the close. The app
			// keeps running in the tray; the user quits from the tray menu.
			//
			// Only honored where a tray icon actually exists to restore the window
			// from (Windows). On builds without a tray (Linux/macOS — see
			// trayAvailable / tray_others.go) fall through to the normal
			// confirm/quit path so the window can never vanish with no way to bring
			// it back.
			if ds.CloseToTray && trayAvailable {
				runtime.WindowHide(ctx)
				return true // prevent close
			}

			if !ds.ConfirmOnExit {
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

	// Safe-port handshake: bind the loopback API (preferring :8080, falling back
	// to a dynamic port) BEFORE serving, so the resolved base URL is known and can
	// be handed to the frontend via App.GetApiBase(). Then serve in the background.
	base, err := server.Listen()
	if err != nil {
		log.Println("Error binding API server:", err)
	} else {
		app.apiBase = base
	}
	go server.Serve()

	if err := wailsApp.Run(); err != nil {
		println("Error:", err.Error())
	}
}
