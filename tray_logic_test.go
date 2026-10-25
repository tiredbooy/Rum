package main

import (
	"errors"
	"testing"
	"time"
)

func TestSNIHostUsable(t *testing.T) {
	cases := []struct {
		name       string
		watcher    bool
		registered bool
		err        error
		want       bool
	}{
		{"host registered", true, true, nil, true},
		{"watcher without host", true, false, nil, false},
		{"no watcher on bus", false, false, nil, false},
		{"bus error", true, true, errors.New("no bus"), false},
	}
	for _, c := range cases {
		if got := sniHostUsable(c.watcher, c.registered, c.err); got != c.want {
			t.Errorf("%s: got %v want %v", c.name, got, c.want)
		}
	}
}

func TestShouldHideOnClose(t *testing.T) {
	if !shouldHideOnClose(true, true) {
		t.Error("setting on and tray alive must hide")
	}
	if shouldHideOnClose(true, false) {
		t.Error("tray gone must fall back to normal close")
	}
	if shouldHideOnClose(false, true) {
		t.Error("setting off must not hide")
	}
}

type fakeTray struct{ calls []string }

func (f *fakeTray) Show()      { f.calls = append(f.calls, "show") }
func (f *fakeTray) PauseAll()  { f.calls = append(f.calls, "pause") }
func (f *fakeTray) ResumeAll() { f.calls = append(f.calls, "resume") }
func (f *fakeTray) Quit()      { f.calls = append(f.calls, "quit") }

func TestDispatchTrayAction(t *testing.T) {
	for act, want := range map[trayAction]string{
		trayShow: "show", trayPauseAll: "pause", trayResumeAll: "resume", trayQuit: "quit",
	} {
		f := &fakeTray{}
		dispatchTrayAction(act, f)
		if len(f.calls) != 1 || f.calls[0] != want {
			t.Errorf("action %v: calls=%v want [%s]", act, f.calls, want)
		}
	}
	f := &fakeTray{}
	dispatchTrayAction(trayAction(99), f)
	if len(f.calls) != 0 {
		t.Errorf("unknown action must be a no-op, got %v", f.calls)
	}
}

func TestCachedProbeReusesResultWithinTTL(t *testing.T) {
	now := time.Unix(0, 0)
	calls := 0
	c := &cachedProbe{ttl: 5 * time.Second, now: func() time.Time { return now },
		probe: func() bool { calls++; return calls == 1 }}
	if !c.Get() || !c.Get() || calls != 1 {
		t.Fatalf("within TTL must reuse first result, calls=%d", calls)
	}
	now = now.Add(6 * time.Second)
	if c.Get() || calls != 2 {
		t.Fatalf("after TTL must re-probe, calls=%d", calls)
	}
}
