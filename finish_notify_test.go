package main

import (
	"context"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestFinishMessage(t *testing.T) {
	_, b := finishMessage(finishPending{Action: "shutdown", SecondsLeft: 60})
	if b != "Downloads finished — shutting down in 60s" {
		t.Fatalf("shutdown body = %q", b)
	}
	_, b = finishMessage(finishPending{Action: "sleep", SecondsLeft: 45})
	if b != "Downloads finished — sleeping in 45s" {
		t.Fatalf("sleep body = %q", b)
	}
}

func TestNotifyCommand(t *testing.T) {
	evil := `x"; rm -rf ~ #`
	name, args, ok := notifyCommand("linux", "Rum", evil, true)
	if !ok || name != "notify-send" {
		t.Fatalf("linux: %q %v", name, ok)
	}
	want := []string{"--app-name=Rum", "--wait", "--action=default=Show", "--action=cancel=Cancel", "--", "Rum", evil}
	if !reflect.DeepEqual(args, want) {
		t.Fatalf("linux args = %#v", args)
	}
	_, args, _ = notifyCommand("linux", "Rum", "b", false)
	if reflect.DeepEqual(args, want) || len(args) != 4 {
		t.Fatalf("linux no-actions args = %#v", args)
	}
	name, args, ok = notifyCommand("darwin", "Rum", evil, true)
	if !ok || name != "osascript" || args[len(args)-2] != "Rum" || args[len(args)-1] != evil {
		t.Fatalf("darwin: %q %#v", name, args)
	}
	for _, a := range args[:len(args)-2] {
		if a == evil {
			t.Fatal("body interpolated into script")
		}
	}
	if _, _, ok := notifyCommand("windows", "a", "b", true); ok {
		t.Fatal("windows should use the beeep fallback")
	}
}

func TestDarwinDialogCommand(t *testing.T) {
	evil := `x" & (do shell script "id") & "`
	name, args, ok := notifyCommand("darwin", "Rum", evil, true)
	if !ok || name != "osascript" || args[0] != "-e" || args[1] != darwinDialogScript {
		t.Fatalf("darwin dialog: %q %#v", name, args)
	}
	if args[len(args)-1] != evil || !strings.Contains(args[1], "giving up after 60") {
		t.Fatalf("args = %#v", args)
	}
	_, args, _ = notifyCommand("darwin", "Rum", "b", false)
	if args[1] != darwinScript {
		t.Fatal("no-actions should be a plain notification")
	}
}

func TestParseDarwinDialogOutput(t *testing.T) {
	for in, want := range map[string]string{
		"button returned:Cancel Action, gave up:false\n": "cancel",
		"button returned:Continue, gave up:false":        "default",
		"button returned:Continue, gave up:true":         "",
	} {
		if got := parseNotifyAction(in); got != want {
			t.Errorf("parse(%q)=%q want %q", in, got, want)
		}
	}
}

func TestWindowsDialogMapping(t *testing.T) {
	for ret, want := range map[int]string{idYes: "cancel", idNo: "default", 32000: "", 0: ""} {
		if got := windowsDialogAction(ret); got != want {
			t.Errorf("action(%d)=%q want %q", ret, got, want)
		}
	}
	if windowsDialogFlags&mbTopmost == 0 || windowsDialogFlags&mbYesNo == 0 {
		t.Fatal("flags must be top-most Yes/No")
	}
	if !strings.HasSuffix(windowsDialogText("b"), "Cancel it?") {
		t.Fatal("text must ask the question")
	}
}

func TestWindowsCancelActionCancelsFinish(t *testing.T) {
	var cancelled, shown int
	h := finishActionHandler{cancel: func() { cancelled++ }, show: func() { shown++ }}
	send := func(context.Context, string, string) (string, error) { return windowsDialogAction(idYes), nil }
	deliverFinishNotice(context.Background(), finishPending{Action: "shutdown", SecondsLeft: 60}, send, h)
	if cancelled != 1 || shown != 0 {
		t.Fatalf("cancelled=%d shown=%d", cancelled, shown)
	}
}

func TestParseNotifyAction(t *testing.T) {
	for in, want := range map[string]string{"cancel\n": "cancel", "default\n": "default", "": "", "weird": ""} {
		if got := parseNotifyAction(in); got != want {
			t.Errorf("parse(%q)=%q want %q", in, got, want)
		}
	}
}

func TestCountdownNotifiesOncePerCountdown(t *testing.T) {
	var n int
	last := int64(0)
	step := func(p *finishPending) {
		var fire bool
		fire, last = countdownStarted(last, p)
		if fire {
			n++
		}
	}
	step(nil)
	p := &finishPending{Action: "shutdown", DeadlineMs: 1000}
	step(p)
	step(p)
	step(nil) // cancelled
	step(&finishPending{Action: "shutdown", DeadlineMs: 2000})
	if n != 2 {
		t.Fatalf("fired %d times, want 2", n)
	}
}

func TestWatchFinishCountdownNotifiesOnceAndCancels(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	var mu sync.Mutex
	var notified, cancelled int
	done := make(chan struct{}, 1)
	w := finishWatcher{
		interval: time.Millisecond,
		fetch: func(context.Context) (*finishPending, bool) {
			return &finishPending{Action: "shutdown", DeadlineMs: 5, SecondsLeft: 60}, true
		},
		notify: func(context.Context, finishPending) {
			mu.Lock()
			notified++
			mu.Unlock()
			done <- struct{}{}
		},
	}
	go w.run(ctx)
	<-done
	time.Sleep(20 * time.Millisecond)
	mu.Lock()
	if notified != 1 {
		t.Fatalf("notified %d times", notified)
	}
	mu.Unlock()

	// cancel path: the "cancel" action cancels, anything else shows the window.
	h := finishActionHandler{cancel: func() { cancelled++ }, show: func() { notified += 100 }}
	h.handle("cancel")
	h.handle("default")
	h.handle("")
	if cancelled != 1 || notified != 201 {
		t.Fatalf("cancelled=%d notified=%d", cancelled, notified)
	}
}

func TestDeliverFallsBackToShowWindow(t *testing.T) {
	shown := 0
	deliverFinishNotice(context.Background(), finishPending{Action: "sleep", SecondsLeft: 60},
		func(context.Context, string, string) (string, error) { return "", context.DeadlineExceeded },
		finishActionHandler{cancel: func() {}, show: func() { shown++ }})
	if shown != 1 {
		t.Fatalf("shown=%d", shown)
	}
}

func TestWatcherIgnoresTransientFetchErrors(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	var mu sync.Mutex
	calls, notified := 0, 0
	w := finishWatcher{
		interval: time.Millisecond,
		fetch: func(context.Context) (*finishPending, bool) {
			mu.Lock()
			defer mu.Unlock()
			calls++
			if calls%2 == 0 {
				return nil, false
			}
			return &finishPending{Action: "shutdown", DeadlineMs: 7, SecondsLeft: 60}, true
		},
		notify: func(context.Context, finishPending) { mu.Lock(); notified++; mu.Unlock() },
	}
	go w.run(ctx)
	time.Sleep(60 * time.Millisecond)
	mu.Lock()
	defer mu.Unlock()
	if calls < 4 || notified != 1 {
		t.Fatalf("calls=%d notified=%d, want one notification", calls, notified)
	}
}

func TestDeliverSkipsWindowWhenShuttingDown(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	shown := 0
	deliverFinishNotice(ctx, finishPending{Action: "sleep", SecondsLeft: 60},
		func(context.Context, string, string) (string, error) { return "", context.Canceled },
		finishActionHandler{cancel: func() {}, show: func() { shown++ }})
	if shown != 0 {
		t.Fatalf("shown=%d", shown)
	}
}
