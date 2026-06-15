package main

import (
	"context"
	_ "embed"

	"github.com/getlantern/systray"
	"github.com/wailsapp/wails/v2/pkg/runtime"
)

//go:embed frontend/src/assets/tray-icon.png
var trayIcon []byte // embed a 16x16 or 24x24 PNG

func startTray(ctx context.Context) {
	// systray.Run is blocking, so run it in its own goroutine
	go func() {
		systray.Run(func() {
			systray.SetIcon(trayIcon)
			systray.SetTitle("Rum")
			systray.SetTooltip("Rum Download Manager")

			mShow := systray.AddMenuItem("Show Window", "Bring the app to front")
			mQuit := systray.AddMenuItem("Quit", "Close the application")

			// Menu events loop
			go func() {
				for {
					select {
					case <-mShow.ClickedCh:
						runtime.WindowShow(ctx)
					case <-mQuit.ClickedCh:
						runtime.Quit(ctx)
					}
				}
			}()
		}, func() {
			// optional: cleanup when systray exits
		})
	}()
}
