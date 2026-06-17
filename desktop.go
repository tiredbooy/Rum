package main

import (
	"context"
	"encoding/json"
	"io"
	"log"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/gen2brain/beeep"
	"github.com/wailsapp/wails/v2/pkg/runtime"
)

// completionPollInterval is how often the notifier polls the engine for newly
// completed downloads. A few seconds keeps notifications timely without hammering
// the loopback API.
const completionPollInterval = 3 * time.Second

// clipboardPollInterval is how often the opt-in clipboard watcher samples the
// clipboard (~1.5s per the spec).
const clipboardPollInterval = 1500 * time.Millisecond

// minimizePollInterval is how often the best-effort minimize-to-tray watcher
// samples the window state (Wails v2.12 exposes no OnMinimise callback).
const minimizePollInterval = 400 * time.Millisecond

// httpClient is shared by the desktop pollers; the short timeout keeps a stuck
// request from blocking a poll cycle.
var httpClient = &http.Client{Timeout: 5 * time.Second}

// downloadItem is the minimal subset of the engine's DownloadResponse the
// notifier needs. Field tags match backend/internal/pkg/api/dto.DownloadResponse.
type downloadItem struct {
	ID       string `json:"id"`
	Filename string `json:"filename"`
	Status   string `json:"status"`
}

// watchCompletions polls {APIBase}/api/v1/downloads?status=completed and fires a
// native notification (via beeep) for each download id seen completed for the
// first time. It is leak-free: the loop returns as soon as ctx is cancelled.
//
// Settings are re-read each tick so toggling "Silent" at runtime is honored
// without a restart. The seen-set is seeded on the first poll so downloads that
// were already complete before launch do not spam notifications.
func (a *App) watchCompletions(ctx context.Context) {
	ticker := time.NewTicker(completionPollInterval)
	defer ticker.Stop()

	seen := make(map[string]struct{})
	first := true

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			items := a.fetchCompleted(ctx)
			if items == nil {
				continue
			}

			silent := loadDesktopSettings().Silent

			for _, it := range items {
				if it.ID == "" {
					continue
				}
				if _, ok := seen[it.ID]; ok {
					continue
				}
				seen[it.ID] = struct{}{}
				// Don't notify for downloads already finished before we started.
				if first || silent {
					continue
				}
				name := it.Filename
				if name == "" {
					name = "Download"
				}
				if err := beeep.Notify("Rum", name+" finished", ""); err != nil {
					log.Printf("desktop: notification failed: %v", err)
				}
			}
			first = false
		}
	}
}

// fetchCompleted GETs the completed-downloads list and decodes it. The endpoint
// returns either a bare JSON array of downloads or {"message": "..."} when empty
// (see handlers.GetAllJobs); both are handled. A nil return means "skip this
// tick" (network error or empty/non-array body).
func (a *App) fetchCompleted(ctx context.Context) []downloadItem {
	base := a.GetApiBase()
	if base == "" {
		return nil
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, base+"/api/v1/downloads?status=completed", nil)
	if err != nil {
		return nil
	}
	resp, err := httpClient.Do(req)
	if err != nil {
		return nil
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil
	}

	body, err := io.ReadAll(io.LimitReader(resp.Body, 8<<20)) // cap at 8 MiB
	if err != nil {
		return nil
	}

	var items []downloadItem
	if err := json.Unmarshal(body, &items); err != nil {
		// Empty-state shape is {"message": "No Job Found"} — not an array. Treat as
		// "nothing completed".
		return nil
	}
	return items
}

// watchClipboard polls the clipboard (opt-in via EnableClipboardWatch) and emits
// a Wails "clipboard:url" event the frontend listens for whenever a newly-seen
// http(s) URL appears. Leak-free: returns on ctx cancellation. The enable flag is
// re-read each tick so toggling the preference takes effect without a restart.
func (a *App) watchClipboard(ctx context.Context) {
	ticker := time.NewTicker(clipboardPollInterval)
	defer ticker.Stop()

	var last string

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if !loadDesktopSettings().EnableClipboardWatch {
				continue
			}
			text, err := runtime.ClipboardGetText(a.ctx)
			if err != nil || text == "" {
				continue
			}
			text = strings.TrimSpace(text)
			if text == last {
				continue
			}
			last = text
			if !isHTTPURL(text) {
				continue
			}
			runtime.EventsEmit(a.ctx, "clipboard:url", text)
		}
	}
}

// isHTTPURL reports whether s is a single, well-formed http(s) URL with a host.
func isHTTPURL(s string) bool {
	if strings.ContainsAny(s, " \t\r\n") {
		return false // a multi-line clipboard isn't a single link
	}
	u, err := url.Parse(s)
	if err != nil {
		return false
	}
	return (u.Scheme == "http" || u.Scheme == "https") && u.Host != ""
}

// watchMinimize implements best-effort minimize-to-tray. Wails v2.12 has no
// OnMinimise lifecycle callback, so when MinimizeToTray is enabled we poll
// WindowIsMinimised and hide the window the moment it minimises (which removes it
// from the taskbar, leaving only the tray icon). Leak-free: returns on ctx
// cancellation.
//
// NOTE: this is the documented best-effort path; close-to-tray (handled in
// main.go's OnBeforeClose) is the fully-supported behavior.
func (a *App) watchMinimize(ctx context.Context) {
	ticker := time.NewTicker(minimizePollInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if !loadDesktopSettings().MinimizeToTray {
				continue
			}
			if runtime.WindowIsMinimised(a.ctx) {
				// Un-minimise first so the next WindowShow restores cleanly, then hide.
				runtime.WindowUnminimise(a.ctx)
				runtime.WindowHide(a.ctx)
			}
		}
	}
}

// restoreWindowState applies the persisted geometry on startup. Zero values mean
// "no saved state", so it no-ops in that case. Maximized takes precedence over an
// explicit size/position.
func restoreWindowState(ctx context.Context) {
	ws := loadDesktopSettings().WindowState
	if ws == (windowState{}) {
		return
	}
	if ws.W > 0 && ws.H > 0 {
		runtime.WindowSetSize(ctx, ws.W, ws.H)
	}
	// Only set a position if one was stored (X/Y of 0,0 is also a valid position,
	// but we treat an all-zero WindowState above as "unset").
	runtime.WindowSetPosition(ctx, ws.X, ws.Y)
	if ws.Maximized {
		runtime.WindowMaximise(ctx)
	}
}

// persistWindowState captures the current geometry and writes it back to
// settings.json. Called from OnBeforeClose so the next launch restores it.
func persistWindowState(ctx context.Context, ds *desktopSettings) {
	maximized := runtime.WindowIsMaximised(ctx)
	w, h := runtime.WindowGetSize(ctx)
	x, y := runtime.WindowGetPosition(ctx)

	ws := windowState{W: w, H: h, X: x, Y: y, Maximized: maximized}
	if err := ds.SaveWindowState(ws); err != nil {
		log.Printf("desktop: failed to persist window state: %v", err)
	}
}

// reconcileAutostart makes the OS login-autostart entry match the persisted
// LaunchOnStartup preference on every startup. SetAutostart is implemented
// per-OS in autostart_{linux,windows,darwin}.go. Failures are logged, not fatal.
func reconcileAutostart() {
	enable := loadDesktopSettings().LaunchOnStartup
	if err := SetAutostart(enable); err != nil {
		log.Printf("desktop: reconcile autostart (enable=%v): %v", enable, err)
	}
}

// postNoBody issues a fire-and-forget POST (no request body, response discarded)
// to the given API path. Used by the tray quick actions. Errors are logged, not
// surfaced.
func postNoBody(base, path string) {
	if base == "" {
		return
	}
	req, err := http.NewRequest(http.MethodPost, base+path, nil)
	if err != nil {
		log.Printf("desktop: build request %s: %v", path, err)
		return
	}
	resp, err := httpClient.Do(req)
	if err != nil {
		log.Printf("desktop: POST %s: %v", path, err)
		return
	}
	_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, 1<<20))
	resp.Body.Close()
}
