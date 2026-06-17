//go:build windows

package main

import (
	"context"
	_ "embed"

	"github.com/getlantern/systray"
	"github.com/wailsapp/wails/v2/pkg/runtime"
)

// trayAvailable reports whether this build runs a real system tray. Only the
// Windows build does. getlantern/systray drives its own native UI event loop;
// on Windows that loop is just a hidden message-pump window, which coexists
// fine with Wails' WebView2. On Linux it is a second GTK main loop and on macOS
// a second AppKit run loop — and Wails already owns one of those on the main
// thread. Two native UI loops in one process race on shared global state and
// abort (SIGABRT inside gtk_widget_show_all on Linux). See tray_others.go for
// the no-op used everywhere else.
const trayAvailable = true

//go:embed frontend/src/assets/tray-icon.png
var trayIcon []byte // embed a 16x16 or 24x24 PNG

// startTray launches the system tray with quick actions. It is called from
// App.startup (the tray was previously dead code — defined but never invoked).
//
// systray.Run is blocking, so it runs in its own goroutine. The menu event loop
// is a second goroutine that exits cleanly when the user picks Quit (it closes a
// done channel and returns) or when the tray itself shuts down — no leaked
// goroutine.
func startTray(ctx context.Context, apiBase string) {
	go func() {
		systray.Run(func() {
			systray.SetIcon(trayIcon)
			systray.SetTitle("Rum")
			systray.SetTooltip("Rum Download Manager")

			mShow := systray.AddMenuItem("Show Window", "Bring the app to front")
			systray.AddSeparator()
			mStartAll := systray.AddMenuItem("Start all", "Start all downloads")
			mPauseAll := systray.AddMenuItem("Pause all", "Pause all downloads")
			systray.AddSeparator()
			mQuit := systray.AddMenuItem("Quit", "Close the application")

			done := make(chan struct{})

			// Menu events loop. Returns when Quit is chosen (done closed) so it does
			// not outlive the app.
			go func() {
				for {
					select {
					case <-done:
						return
					case <-mShow.ClickedCh:
						runtime.WindowShow(ctx)
					case <-mStartAll.ClickedCh:
						postNoBody(apiBase, "/api/v1/downloads/start-all")
					case <-mPauseAll.ClickedCh:
						postNoBody(apiBase, "/api/v1/downloads/pause-all")
					case <-mQuit.ClickedCh:
						close(done)
						// Mark intentional quit so OnBeforeClose lets the app exit
						// instead of hiding to tray.
						isQuitting = true
						runtime.Quit(ctx)
						return
					}
				}
			}()
		}, func() {
			// Cleanup when systray exits; nothing to release here.
		})
	}()
}
