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
	"strings"
	"time"

	"github.com/gen2brain/beeep"
	wruntime "github.com/wailsapp/wails/v2/pkg/runtime"
)

const finishPollInterval = 2 * time.Second

// finishPending mirrors download.FinishPending as served by GET /finish-action.
type finishPending struct {
	Action      string `json:"action"`
	DeadlineMs  int64  `json:"deadline_ms"`
	SecondsLeft int    `json:"seconds_left"`
}

// finishMessage builds the notification title and body for a countdown.
func finishMessage(p finishPending) (title, body string) {
	verb := "shutting down"
	if p.Action == "sleep" {
		verb = "sleeping"
	}
	return "Rum", fmt.Sprintf("Downloads finished — %s in %ds", verb, p.SecondsLeft)
}

// darwinScript reads title/body from argv, so no text is spliced into code.
const darwinScript = `on run argv
display notification (item 2 of argv) with title (item 1 of argv)
end run`

// notifyCommand returns the argv for a native notification on goos. ok is false
// where the beeep fallback should be used instead (Windows and others). Text is
// passed as separate args, never through a shell.
func notifyCommand(goos, title, body string, actions bool) (name string, args []string, ok bool) {
	switch goos {
	case "linux":
		args = []string{"--app-name=Rum"}
		if actions {
			args = append(args, "--wait", "--action=default=Show", "--action=cancel=Cancel")
		}
		return "notify-send", append(args, "--", title, body), true
	case "darwin":
		return "osascript", []string{"-e", darwinScript, title, body}, true
	}
	return "", nil, false
}

// parseNotifyAction maps notify-send --wait output to a known action id.
func parseNotifyAction(out string) string {
	switch s := strings.TrimSpace(out); s {
	case "default", "cancel":
		return s
	}
	return ""
}

// countdownStarted reports whether p is a countdown not seen before (keyed by
// its deadline) and returns the new last-seen deadline.
func countdownStarted(last int64, p *finishPending) (bool, int64) {
	if p == nil {
		return false, 0
	}
	return p.DeadlineMs != last, p.DeadlineMs
}

// finishActionHandler routes a clicked notification action.
type finishActionHandler struct{ cancel, show func() }

func (h finishActionHandler) handle(action string) {
	if action == "cancel" {
		h.cancel()
		return
	}
	h.show()
}

// sendFunc shows a notification and returns the chosen action id, if any.
type sendFunc func(ctx context.Context, title, body string) (string, error)

// deliverFinishNotice notifies; if that fails the window is raised instead so
// the countdown dialog is never hidden.
func deliverFinishNotice(ctx context.Context, p finishPending, send sendFunc, h finishActionHandler) {
	title, body := finishMessage(p)
	action, err := send(ctx, title, body)
	if ctx.Err() != nil {
		return // app is shutting down; do not raise a window or re-notify
	}
	if err != nil {
		log.Printf("desktop: finish notification failed, showing window: %v", err)
		h.show()
		return
	}
	if action != "" {
		h.handle(action)
	}
}

// systemSend uses notify-send/osascript when present, else beeep.
func systemSend(ctx context.Context, title, body string) (string, error) {
	if name, args, ok := notifyCommand(runtime.GOOS, title, body, true); ok {
		if _, err := exec.LookPath(name); err == nil {
			cctx, cancel := context.WithTimeout(ctx, finishCountdownWait)
			defer cancel()
			var out bytes.Buffer
			cmd := exec.CommandContext(cctx, name, args...)
			cmd.Stdout = &out
			if err := cmd.Run(); err == nil {
				return parseNotifyAction(out.String()), nil
			}
			// Older notify-send lacks --action: retry without actions.
			if _, plain, _ := notifyCommand(runtime.GOOS, title, body, false); name == "notify-send" {
				if err := exec.CommandContext(ctx, name, plain...).Run(); err == nil {
					return "", nil
				}
			}
		}
	}
	return "", beeep.Notify(title, body, "")
}

const finishCountdownWait = 65 * time.Second

type finishWatcher struct {
	interval time.Duration
	fetch    func(context.Context) (*finishPending, bool)
	notify   func(context.Context, finishPending)
}

// run polls and notifies once per countdown. Returns on ctx cancellation.
func (w finishWatcher) run(ctx context.Context) {
	t := time.NewTicker(w.interval)
	defer t.Stop()
	var last int64
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			p, ok := w.fetch(ctx)
			if !ok {
				continue // transient API error: keep the last countdown so it is not re-announced
			}
			var fire bool
			fire, last = countdownStarted(last, p)
			if fire {
				go w.notify(ctx, *p) // notify-send --wait blocks until dismissed
			}
		}
	}
}

// fetchFinishPending returns (pending, ok); ok is false when the API was unreachable.
func (a *App) fetchFinishPending(ctx context.Context) (*finishPending, bool) {
	base := a.GetApiBase()
	if base == "" {
		return nil, false
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, base+"/api/v1/finish-action", nil)
	if err != nil {
		return nil, false
	}
	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, false
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil || resp.StatusCode != http.StatusOK {
		return nil, false
	}
	var r struct {
		Pending *finishPending `json:"pending"`
	}
	if json.Unmarshal(body, &r) != nil {
		return nil, false
	}
	return r.Pending, true
}

// watchFinishCountdown surfaces the sleep/shutdown countdown natively, since the
// in-app dialog is invisible while Rum is hidden in the tray.
func (a *App) watchFinishCountdown(ctx context.Context) {
	h := finishActionHandler{
		cancel: func() { postNoBody(a.GetApiBase(), "/api/v1/finish-action/cancel") },
		show: func() {
			wruntime.WindowShow(a.ctx)
			wruntime.WindowUnminimise(a.ctx)
		},
	}
	finishWatcher{
		interval: finishPollInterval,
		fetch:    a.fetchFinishPending,
		notify: func(c context.Context, p finishPending) {
			deliverFinishNotice(c, p, systemSend, h)
		},
	}.run(ctx)
}
