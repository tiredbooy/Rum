package main

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

const gib = int64(1) << 30

func TestWindowVisibilityTracker(t *testing.T) {
	var v windowVisibility
	if v.Hidden() {
		t.Fatal("a fresh window counts as visible")
	}
	v.SetHidden(true)
	if !v.Hidden() {
		t.Fatal("tray-hidden window must report hidden")
	}
	v.Shown()
	if v.Hidden() {
		t.Fatal("Shown must clear hidden")
	}
	v.SetMinimised(true)
	if !v.Hidden() {
		t.Fatal("minimised window must report hidden")
	}
	v.SetMinimised(false)
	v.SetPageVisible(false)
	if !v.Hidden() {
		t.Fatal("page reporting not visible must count as hidden")
	}
	v.SetPageVisible(true)
	if v.Hidden() {
		t.Fatal("page visible again clears the fallback")
	}
	v.SetHidden(true)
	v.SetMinimised(true)
	v.SetPageVisible(false)
	v.Shown()
	if v.Hidden() {
		t.Fatal("Shown clears every source")
	}
}

func TestFormatNeeded(t *testing.T) {
	for in, want := range map[int64]string{
		gib * 5 / 2:    "2.5 GB",
		300 << 20:      "300 MB",
		gib:            "1.0 GB",
		0:              "",
		-5:             "",
		1 << 10:        "1 MB", // never rounds down to 0
		gib*12 + 1<<29: "12.5 GB",
	} {
		if got := formatNeeded(in); got != want {
			t.Errorf("formatNeeded(%d)=%q want %q", in, got, want)
		}
	}
}

func disk(id string, need int64) downloadItem {
	return downloadItem{ID: id, Status: "error", ErrorCode: "disk_full", NeededBytes: need}
}

func failed(id, name, code string) downloadItem {
	return downloadItem{ID: id, Filename: name, Status: "error", ErrorCode: code}
}

func newTestNotifier(clock *time.Time) *hiddenNotifier {
	return newHiddenNotifier(func() time.Time { return *clock })
}

func TestNotifierSeedsOnFirstObservation(t *testing.T) {
	now := time.Unix(1000, 0)
	n := newTestNotifier(&now)
	if notes := n.observe([]downloadItem{disk("a", gib), failed("b", "x", "tls")}); len(notes) != 0 {
		t.Fatalf("pre-existing failures must not notify: %v", notes)
	}
	if notes := n.observe([]downloadItem{disk("a", gib), failed("b", "x", "tls")}); len(notes) != 0 {
		t.Fatalf("seeded failures must stay quiet: %v", notes)
	}
}

func TestNotifierDiskFullCoalescesAndNamesSize(t *testing.T) {
	now := time.Unix(1000, 0)
	n := newTestNotifier(&now)
	n.observe(nil)
	notes := n.observe([]downloadItem{disk("a", gib*5/2), disk("b", gib), disk("c", gib)})
	if len(notes) != 1 {
		t.Fatalf("want 1 coalesced disk-full note, got %d: %v", len(notes), notes)
	}
	if !strings.Contains(notes[0].Body, "2.5 GB") || notes[0].Key != "disk_full" {
		t.Fatalf("note = %+v", notes[0])
	}
}

func TestNotifierDiskFullRateLimited(t *testing.T) {
	now := time.Unix(1000, 0)
	n := newTestNotifier(&now)
	n.observe(nil)
	if len(n.observe([]downloadItem{disk("a", gib)})) != 1 {
		t.Fatal("first disk-full must notify")
	}
	now = now.Add(30 * time.Second)
	if notes := n.observe([]downloadItem{disk("a", gib), disk("b", gib)}); len(notes) != 0 {
		t.Fatalf("second disk-full inside the window must coalesce: %v", notes)
	}
	now = now.Add(diskFullCooldown + time.Second)
	if len(n.observe([]downloadItem{disk("a", gib), disk("b", gib), disk("c", gib)})) != 1 {
		t.Fatal("a new disk-full after the cooldown must notify")
	}
}

func TestNotifierFailureOncePerJobAndState(t *testing.T) {
	now := time.Unix(1000, 0)
	n := newTestNotifier(&now)
	n.observe(nil)
	retry := failed("r", "r.bin", "network_down")
	retry.Retryable = true
	notes := n.observe([]downloadItem{failed("a", "a.iso", "tls"), retry})
	if len(notes) != 1 || notes[0].Key != "failed" || !strings.Contains(notes[0].Body, "a.iso") {
		t.Fatalf("want one non-retryable failure note, got %v", notes)
	}
	if notes := n.observe([]downloadItem{failed("a", "a.iso", "tls")}); len(notes) != 0 {
		t.Fatalf("same job+state must not repeat: %v", notes)
	}
	// Retried then failed again: a new state, so it notifies once more.
	n.observe([]downloadItem{{ID: "a", Status: "running"}})
	if len(n.observe([]downloadItem{failed("a", "a.iso", "tls")})) != 1 {
		t.Fatal("a fresh failure after a retry must notify")
	}
}

func TestNotifierFailuresCoalesceInOneTick(t *testing.T) {
	now := time.Unix(1000, 0)
	n := newTestNotifier(&now)
	n.observe(nil)
	notes := n.observe([]downloadItem{failed("a", "a", "tls"), failed("b", "b", "unknown"), failed("c", "c", "permission")})
	if len(notes) != 1 || !strings.Contains(notes[0].Body, "3") {
		t.Fatalf("want one note mentioning 3 failures, got %v", notes)
	}
}

func TestNotifierAllFinished(t *testing.T) {
	now := time.Unix(1000, 0)
	n := newTestNotifier(&now)
	run := func(id, st string) downloadItem { return downloadItem{ID: id, Status: st} }
	n.observe([]downloadItem{run("a", "running"), run("b", "pending")})
	if notes := n.observe([]downloadItem{run("a", "completed"), run("b", "running")}); len(notes) != 0 {
		t.Fatalf("still active: %v", notes)
	}
	notes := n.observe([]downloadItem{run("a", "completed"), run("b", "completed")})
	if len(notes) != 1 || notes[0].Key != "all_done" {
		t.Fatalf("want all_done, got %v", notes)
	}
	if notes := n.observe([]downloadItem{run("a", "completed"), run("b", "completed")}); len(notes) != 0 {
		t.Fatalf("all_done must fire once: %v", notes)
	}
}

func TestNotifierPauseIsNotFinished(t *testing.T) {
	now := time.Unix(1000, 0)
	n := newTestNotifier(&now)
	n.observe([]downloadItem{{ID: "a", Status: "running"}})
	if notes := n.observe([]downloadItem{{ID: "a", Status: "paused"}}); len(notes) != 0 {
		t.Fatalf("pausing is not finishing: %v", notes)
	}
}

func TestHiddenWatcherTickGating(t *testing.T) {
	now := time.Unix(1000, 0)
	var sent []hiddenNote
	hidden, enabled := false, true
	items := []downloadItem{}
	w := hiddenWatcher{
		notifier: newTestNotifier(&now),
		fetch:    func(context.Context) ([]downloadItem, bool) { return items, true },
		hidden:   func() bool { return hidden },
		enabled:  func() bool { return enabled },
		send:     func(n hiddenNote) { sent = append(sent, n) },
	}
	ctx := context.Background()
	w.tick(ctx) // visible: nothing
	hidden = true
	w.tick(ctx) // hidden, seeds
	items = []downloadItem{disk("a", gib)}
	enabled = false
	w.tick(ctx) // disabled: nothing, and drops state
	if len(sent) != 0 {
		t.Fatalf("sent while disabled/seeding: %v", sent)
	}
	enabled = true
	w.tick(ctx) // re-seed with the existing failure: quiet
	if len(sent) != 0 {
		t.Fatalf("stale failure notified after re-enable: %v", sent)
	}
	items = append(items, disk("b", 2*gib))
	w.tick(ctx)
	if len(sent) != 1 || !strings.Contains(sent[0].Body, "2.0 GB") {
		t.Fatalf("want one disk-full send, got %v", sent)
	}
	hidden = false
	items = append(items, failed("c", "c", "tls"))
	w.tick(ctx)
	hidden = true
	w.tick(ctx) // re-seed after being visible: the failure seen while visible is quiet
	if len(sent) != 1 {
		t.Fatalf("failure while visible leaked a notification: %v", sent)
	}
}

func TestDesktopSettingsNotifyWhenHiddenDefaultsOn(t *testing.T) {
	t.Setenv("RUM_CONFIG_DIR", t.TempDir())
	if !loadDesktopSettings().NotifyWhenHidden {
		t.Fatal("missing settings.json must default notify_when_hidden on")
	}
	writeTestSettings(t, `{"silent":false}`)
	if !loadDesktopSettings().NotifyWhenHidden {
		t.Fatal("absent key must default on")
	}
	writeTestSettings(t, `{"notify_when_hidden":false}`)
	if loadDesktopSettings().NotifyWhenHidden {
		t.Fatal("explicit false must be kept")
	}
}

func writeTestSettings(t *testing.T, body string) {
	t.Helper()
	path := settingsPath()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}
