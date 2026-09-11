package main

import (
	"context"
	"fmt"
	"log"
	goruntime "runtime"

	"github.com/tiredbooy/Rum/backend/cmd/server"
	"github.com/wailsapp/wails/v2/pkg/runtime"
)

// App is the Wails-bound application object. Its exported methods are auto-bound
// (via Bind: []interface{}{app}) and callable from the frontend as
// window.go.main.App.<Method>.
type App struct {
	ctx context.Context

	// apiBase is the resolved loopback API base URL (e.g. "http://127.0.0.1:8081"),
	// stashed by main() after server.Listen(). The frontend reads it via
	// GetApiBase() to talk to the engine even when 8080 is taken.
	apiBase string

	// bgCancel cancels every background goroutine the desktop layer starts
	// (completion notifier, clipboard watcher, minimize-to-tray watcher). It is
	// set in startup and called in shutdown so nothing leaks past app exit.
	bgCancel context.CancelFunc
}

func NewApp() *App {
	return &App{}
}

// startup is wired to options.App.OnStartup. It captures the Wails context and
// kicks off the desktop background features (all cancelable via bgCancel).
func (a *App) startup(ctx context.Context) {
	a.ctx = ctx

	// Root context for all desktop background goroutines, cancelled in shutdown.
	bgCtx, cancel := context.WithCancel(ctx)
	a.bgCancel = cancel

	// System tray with quick actions (Show / Start all / Pause all / Quit).
	startTray(ctx, a.GetApiBase())

	// Restore the persisted window geometry (best effort; no-op if unset).
	restoreWindowState(ctx)

	// Reconcile OS login-autostart with the persisted preference.
	reconcileAutostart()

	// Native notifications on newly-completed downloads (leak-free poller).
	go a.watchCompletions(bgCtx)

	// Opt-in clipboard watcher: emits "clipboard:url" events for the frontend.
	go a.watchClipboard(bgCtx)

	// Keep the OS login entry in step with the "Launch on startup" toggle while
	// the app runs (reconcileAutostart above only covers launch time).
	go a.watchAutostart(bgCtx)

	// Best-effort minimize-to-tray (Wails v2.12 has no OnMinimise callback). Only
	// started where a tray exists to minimize into — otherwise hiding the window
	// on minimize would strand it with no tray icon to restore it from (see
	// trayAvailable / tray_others.go).
	if trayAvailable {
		go a.watchMinimize(bgCtx)
	}
}

// shutdown is wired to options.App.OnShutdown. It cancels every background
// goroutine so none survive the process exit.
func (a *App) shutdown(ctx context.Context) {
	if a.bgCancel != nil {
		a.bgCancel()
	}
}

// GetApiBase returns the resolved loopback API base URL. It prefers the value
// stashed at startup and falls back to the engine's live accessor.
func (a *App) GetApiBase() string {
	if a.apiBase != "" {
		return a.apiBase
	}
	return server.APIBase()
}

// ChooseDir opens the native folder picker and returns the selected directory.
// An empty string with a nil error means the user cancelled.
//
// Failures are logged before being returned: a rejected promise on the frontend
// used to be swallowed by the caller, so a picker that could not open was
// indistinguishable from a cancel — the button simply "did nothing", with
// nothing in the logs either. The frontend now surfaces the message; this keeps
// a copy where a bug report can find it.
func (a *App) ChooseDir() (string, error) {
	if a.ctx == nil {
		log.Println("desktop: ChooseDir called before startup")
		return "", fmt.Errorf("the window is not ready yet")
	}
	dir, err := runtime.OpenDirectoryDialog(a.ctx, runtime.OpenDialogOptions{
		Title:                "Choose folder",
		CanCreateDirectories: true,
	})
	if err != nil {
		log.Printf("desktop: folder picker failed: %v", err)
		return "", fmt.Errorf("could not open the folder picker")
	}
	return dir, nil
}

// DesktopCapabilities reports which platform-specific desktop features this
// build actually has, so the settings UI can disable a control instead of
// offering a switch that silently does nothing.
type DesktopCapabilities struct {
	// Tray is true only where a real system tray exists (Windows in this build —
	// see trayAvailable / tray_others.go). Minimize-to-tray and close-to-tray are
	// inert without it.
	Tray bool `json:"tray"`
	// FolderPicker is true when the native directory dialog is usable.
	FolderPicker bool `json:"folderPicker"`
	// Platform is the GOOS this build runs on ("linux", "windows", "darwin").
	Platform string `json:"platform"`
}

// Capabilities returns the desktop feature matrix for the running build.
func (a *App) Capabilities() DesktopCapabilities {
	return DesktopCapabilities{
		Tray:         trayAvailable,
		FolderPicker: a.ctx != nil,
		Platform:     goruntime.GOOS,
	}
}

// Greet is retained from the Wails template. Kept (rather than removed) because
// the frontend may still reference window.go.main.App.Greet; it is harmless.
func (a *App) Greet(name string) string {
	return fmt.Sprintf("Hello %s, It's show time!", name)
}
