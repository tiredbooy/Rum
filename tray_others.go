//go:build !windows

package main

import "context"

// trayAvailable is false on every non-Windows platform — see the explanation on
// the Windows const in tray_windows.go. The desktop layer reads this so the
// close-to-tray and minimize-to-tray features degrade gracefully (they revert to
// normal window behavior instead of hiding the window with no tray icon to
// restore it from).
const trayAvailable = false

// startTray is a no-op outside Windows.
//
// WHY: getlantern/systray spins up a second native UI event loop in its own
// goroutine — a GTK main loop on Linux, an AppKit run loop on macOS. Wails
// already runs one of those on the main OS thread for the WebKit window. GTK and
// AppKit are single-threaded; two independent UI loops in one process race on
// global state and abort the whole process with SIGABRT (it crashed inside
// gtk_widget_show_all the moment Wails showed the window). There is no flag to
// make getlantern/systray share Wails' existing loop.
//
// A real cross-platform tray needs Wails v3 (which has native tray support) or an
// app-indicator wired into Wails' own GTK main loop. Until then the tray is
// Windows-only; the ctx/apiBase parameters are accepted to match the Windows
// signature. This is a deliberately silent no-op — the close/minimize-to-tray
// fallbacks (gated on trayAvailable) and the docs/UI already convey the behavior,
// so there's no need to print a startup line that reads like an error.
func startTray(ctx context.Context, apiBase string) {}
