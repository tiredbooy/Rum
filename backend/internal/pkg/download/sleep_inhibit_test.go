package download

import (
	"sync/atomic"
	"testing"
	"time"

	"github.com/tiredbooy/Rum/backend/internal/pkg/config"
	"github.com/tiredbooy/Rum/backend/internal/pkg/power"
)

type countingBackend struct{ held, acquires atomic.Int32 }

func (c *countingBackend) Acquire(string) (func(), error) {
	c.held.Add(1)
	c.acquires.Add(1)
	var done atomic.Bool
	return func() {
		if done.CompareAndSwap(false, true) {
			c.held.Add(-1)
		}
	}, nil
}

func waitHeld(t *testing.T, c *countingBackend, want int32) {
	t.Helper()
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		if c.held.Load() == want {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("held = %d, want %d", c.held.Load(), want)
}

func TestSystemStaysAwakeWhileAnyDownloadRuns(t *testing.T) {
	srv := newEndlessServer(t)
	defer srv.Close()
	m, _ := newManagerForTest(t, 2)
	defer m.Shutdown()

	fb := &countingBackend{}
	in := power.NewInhibitor(fb)
	in.SetEnabled(true)
	m.SetSleepInhibitor(in)

	for _, id := range []string{"a", "b"} {
		m.mu.Lock()
		m.jobs[id] = &Job{ID: id, URL: srv.URL + "/" + id, FileName: id + ".bin",
			TotalSize: -1, Status: StatusPending, Priority: "normal"}
		m.mu.Unlock()
		if err := m.StartJob(nil, id); err != nil {
			t.Fatal(err)
		}
	}
	waitHeld(t, fb, 1) // two running downloads share one inhibition
	if err := m.PauseJob("a"); err != nil {
		t.Fatal(err)
	}
	time.Sleep(150 * time.Millisecond)
	if fb.held.Load() != 1 {
		t.Fatal("released while a download still runs")
	}
	if err := m.PauseJob("b"); err != nil {
		t.Fatal(err)
	}
	waitHeld(t, fb, 0)
	if fb.acquires.Load() != 1 {
		t.Fatalf("acquires = %d, want 1", fb.acquires.Load())
	}
}

func TestApplySettingsTogglesPreventSleep(t *testing.T) {
	srv := newEndlessServer(t)
	defer srv.Close()
	m, _ := newManagerForTest(t, 1)
	defer m.Shutdown()

	fb := &countingBackend{}
	in := power.NewInhibitor(fb)
	in.SetEnabled(true)
	m.SetSleepInhibitor(in)

	m.mu.Lock()
	m.jobs["a"] = &Job{ID: "a", URL: srv.URL + "/a", FileName: "a.bin", TotalSize: -1, Status: StatusPending, Priority: "normal"}
	m.mu.Unlock()
	if err := m.StartJob(nil, "a"); err != nil {
		t.Fatal(err)
	}
	waitHeld(t, fb, 1)

	var s config.Setting
	_ = s.LoadSettingMetadata()
	s.PreventSleep = false
	m.ApplySettings(s)
	waitHeld(t, fb, 0)
	s.PreventSleep = true
	m.ApplySettings(s)
	waitHeld(t, fb, 1)
	_ = m.PauseJob("a")
	waitHeld(t, fb, 0)
}
