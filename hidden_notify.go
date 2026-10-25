package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os/exec"
	"runtime"
	"time"

	"github.com/gen2brain/beeep"
	wruntime "github.com/wailsapp/wails/v2/pkg/runtime"
)

const (
	// hiddenPollInterval is how often downloads are sampled while the window is
	// hidden. Nothing is fetched while it is visible.
	hiddenPollInterval = 3 * time.Second
	// diskFullCooldown coalesces repeated disk-full failures (every queued job
	// fails the same way) into one notification.
	diskFullCooldown = 2 * time.Minute
	// hiddenNoticeWait bounds how long a notification waits for a click.
	hiddenNoticeWait = 30 * time.Second
)

// windowVis is the process-wide window visibility, fed by the hide/show call
// sites (tray, close, minimise watcher, SetWindowVisible).
var windowVis windowVisibility

// hiddenNote is one notification to show. Key names the state it reports.
type hiddenNote struct{ Key, Title, Body string }

// hiddenNotifier turns successive download snapshots into notifications:
// disk full (coalesced, rate-limited), non-retryable failure (once per job and
// state) and all-finished (once per run of work).
type hiddenNotifier struct {
	now        func() time.Time
	primed     bool
	seen       map[string]string // job id -> state already reported
	prevActive map[string]bool
	lastDisk   time.Time
}

func newHiddenNotifier(now func() time.Time) *hiddenNotifier {
	return &hiddenNotifier{now: now, seen: map[string]string{}, prevActive: map[string]bool{}}
}

// reset forgets the snapshot so the next observation re-seeds silently. It runs
// whenever nobody needs notifications, so old failures never replay later.
func (n *hiddenNotifier) reset() {
	n.primed = false
	n.seen = map[string]string{}
	n.prevActive = map[string]bool{}
}

// observe compares items with the previous snapshot. The first call only seeds.
func (n *hiddenNotifier) observe(items []downloadItem) []hiddenNote {
	var diskNeed int64
	var diskNew, failNew int
	var failName string
	active := map[string]bool{}
	done := map[string]bool{}

	for _, it := range items {
		switch it.Status {
		case "pending", "running":
			active[it.ID] = true
		case "completed":
			done[it.ID] = true
		}
		if it.Status != "error" {
			delete(n.seen, it.ID) // a retry resets the state, so a new failure reports
			continue
		}
		state := "failed"
		if it.ErrorCode == "disk_full" {
			state = "disk_full"
		} else if it.Retryable {
			continue // the engine retries these itself
		}
		if n.seen[it.ID] == state {
			continue
		}
		n.seen[it.ID] = state
		if state == "disk_full" {
			diskNew++
			diskNeed = max(diskNeed, it.NeededBytes)
		} else {
			failNew++
			failName = it.Filename
		}
	}

	finished := false
	for id := range n.prevActive {
		if done[id] {
			finished = true
		}
	}
	finished = finished && len(active) == 0
	n.prevActive = active

	if !n.primed {
		n.primed = true
		return nil
	}

	var notes []hiddenNote
	if diskNew > 0 && (n.lastDisk.IsZero() || n.now().Sub(n.lastDisk) >= diskFullCooldown) {
		n.lastDisk = n.now()
		body := "Disk full — free up space"
		if s := formatNeeded(diskNeed); s != "" {
			body = "Disk full — free up " + s
		}
		notes = append(notes, hiddenNote{"disk_full", "Rum", body})
	}
	switch {
	case failNew == 1:
		name := failName
		if name == "" {
			name = "A download"
		}
		notes = append(notes, hiddenNote{"failed", "Rum", name + " failed"})
	case failNew > 1:
		notes = append(notes, hiddenNote{"failed", "Rum", fmt.Sprintf("%d downloads failed", failNew)})
	}
	if finished {
		notes = append(notes, hiddenNote{"all_done", "Rum", "All downloads finished"})
	}
	return notes
}

// formatNeeded renders a byte count as "2.5 GB" / "300 MB"; "" when unknown.
func formatNeeded(b int64) string {
	const mib = int64(1) << 20
	switch {
	case b <= 0:
		return ""
	case b >= 1<<30:
		return fmt.Sprintf("%.1f GB", float64(b)/float64(int64(1)<<30))
	default:
		return fmt.Sprintf("%d MB", max(1, (b+mib-1)/mib))
	}
}

// hiddenWatcher samples downloads while the window is hidden.
type hiddenWatcher struct {
	notifier *hiddenNotifier
	fetch    func(context.Context) ([]downloadItem, bool)
	hidden   func() bool
	enabled  func() bool
	send     func(hiddenNote)
}

func (w hiddenWatcher) tick(ctx context.Context) {
	if !w.enabled() || !w.hidden() {
		w.notifier.reset()
		return
	}
	items, ok := w.fetch(ctx)
	if !ok {
		return
	}
	for _, note := range w.notifier.observe(items) {
		w.send(note)
	}
}

func (w hiddenWatcher) run(ctx context.Context) {
	t := time.NewTicker(hiddenPollInterval)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			w.tick(ctx)
		}
	}
}

// showWindow raises the window and records that it is visible again.
func (a *App) showWindow() {
	windowVis.Shown()
	wruntime.WindowShow(a.ctx)
	wruntime.WindowUnminimise(a.ctx)
}

// SetWindowVisible is the frontend's visibilitychange report, a fallback for
// states the backend cannot observe (minimised, other desktop).
func (a *App) SetWindowVisible(visible bool) {
	windowVis.SetPageVisible(visible)
}

// watchHidden notifies about important events while the window is not visible.
func (a *App) watchHidden(ctx context.Context) {
	hiddenWatcher{
		notifier: newHiddenNotifier(time.Now),
		fetch:    a.fetchAllDownloads,
		hidden: func() bool {
			windowVis.SetMinimised(wruntime.WindowIsMinimised(a.ctx))
			return windowVis.Hidden()
		},
		enabled: func() bool { return loadDesktopSettings().NotifyWhenHidden },
		send: func(n hiddenNote) {
			go sendHiddenNote(ctx, n, a.showWindow)
		},
	}.run(ctx)
}

// fetchAllDownloads returns every download; ok is false when the API failed.
func (a *App) fetchAllDownloads(ctx context.Context) ([]downloadItem, bool) {
	base := a.GetApiBase()
	if base == "" {
		return nil, false
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, base+"/api/v1/downloads", nil)
	if err != nil {
		return nil, false
	}
	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, false
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, false
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, 8<<20))
	if err != nil {
		return nil, false
	}
	var items []downloadItem
	if err := json.Unmarshal(body, &items); err != nil {
		return []downloadItem{}, true // {"message": "No Job Found"}: nothing yet
	}
	return items, true
}

// hiddenNoticeArgs is notifyCommand with a click action that shows the window
// (no Cancel button: these notices have nothing to cancel).
func hiddenNoticeArgs(goos, title, body string) (name string, args []string, ok bool) {
	name, args, ok = notifyCommand(goos, title, body, true)
	if !ok {
		return
	}
	kept := args[:0:0]
	for _, a := range args {
		if a != "--action=cancel=Cancel" {
			kept = append(kept, a)
		}
	}
	return name, kept, true
}

// sendHiddenNote shows the notification and raises the window when clicked.
// Windows (beeep) has no click callback, so there it only informs.
func sendHiddenNote(ctx context.Context, n hiddenNote, show func()) {
	if name, args, ok := hiddenNoticeArgs(runtime.GOOS, n.Title, n.Body); ok {
		if _, err := exec.LookPath(name); err == nil {
			cctx, cancel := context.WithTimeout(ctx, hiddenNoticeWait)
			var out bytes.Buffer
			cmd := exec.CommandContext(cctx, name, args...)
			cmd.Stdout = &out
			err := cmd.Run()
			cancel()
			if ctx.Err() != nil {
				return
			}
			if err == nil {
				if parseNotifyAction(out.String()) == "default" {
					show()
				}
				return
			}
			// Older notify-send lacks --action: fall back to a plain notice.
			if _, plain, _ := notifyCommand(runtime.GOOS, n.Title, n.Body, false); name == "notify-send" {
				if exec.CommandContext(ctx, name, plain...).Run() == nil {
					return
				}
			}
		}
	}
	if err := beeep.Notify(n.Title, n.Body, ""); err != nil {
		log.Printf("desktop: hidden notification failed: %v", err)
	}
}
