package download

import (
	"context"
	"testing"
	"time"
)

// Restarting the app mid-window keeps the window's spent allowance.
func TestNightUsageSurvivesRestart(t *testing.T) {
	p := nightPlan("02:00", "07:00")
	p.CapGB = 1
	m := newNightManager(t, p)
	j, cancelled := runningJob(m, "n", true)
	c1 := NewNightController(m)
	c1.tick(context.Background(), nightIn)
	j.SetDownloaded(nightCapBytes(1) * 6 / 10)
	c1.tick(context.Background(), nightIn.Add(time.Minute))

	// "Restart": a brand new controller, same settings dir.
	c2 := NewNightController(m)
	c2.tick(context.Background(), nightIn.Add(2*time.Minute))
	if c2.used < nightCapBytes(1)*6/10 {
		t.Fatalf("usage lost on restart: used=%d", c2.used)
	}
	j.SetDownloaded(nightCapBytes(1)*6/10 + nightCapBytes(1)*5/10)
	c2.tick(context.Background(), nightIn.Add(3*time.Minute))
	if !cancelled.Load() || !m.nightCapHit.Load() {
		t.Fatalf("restarted controller forgot earlier usage; cap not enforced")
	}
}

// A restart after the cap was spent must not start night jobs again.
func TestNightCapStillHeldAfterRestart(t *testing.T) {
	p := nightPlan("02:00", "07:00")
	p.CapGB = 1
	m := newNightManager(t, p)
	j, _ := runningJob(m, "n", true)
	c1 := NewNightController(m)
	c1.tick(context.Background(), nightIn)
	j.SetDownloaded(nightCapBytes(1) + 1)
	c1.tick(context.Background(), nightIn.Add(time.Minute))

	m.nightCapHit.Store(false)
	j.SetStatus(StatusPaused)
	j.SetNightPaused(true)
	c2 := NewNightController(m)
	c2.tick(context.Background(), nightIn.Add(2*time.Minute))
	if m.sched.Queued() != 0 || !m.nightCapHit.Load() {
		t.Fatalf("job started after restart although cap was spent (queued=%d)", m.sched.Queued())
	}
}

// A new window starts at zero, and a midnight-crossing window is one window
// keyed by its start date.
func TestNightUsageKeyedByWindowStartDate(t *testing.T) {
	p := nightPlan("23:00", "06:00")
	p.CapGB = 1
	m := newNightManager(t, p)
	j, _ := runningJob(m, "n", true)
	evening := time.Date(2025, 1, 10, 23, 30, 0, 0, time.UTC)
	c1 := NewNightController(m)
	c1.tick(context.Background(), evening)
	j.SetDownloaded(nightCapBytes(1) / 4)
	c1.tick(context.Background(), evening.Add(time.Minute))

	win, used := loadNightUsage(c1.usagePath)
	if win != "2025-01-10" || used != nightCapBytes(1)/4 {
		t.Fatalf("persisted window=%q used=%d", win, used)
	}

	// After midnight the same window continues (restart included).
	c2 := NewNightController(m)
	c2.tick(context.Background(), time.Date(2025, 1, 11, 1, 0, 0, 0, time.UTC))
	if c2.used != nightCapBytes(1)/4 {
		t.Fatalf("midnight crossing split the window: used=%d", c2.used)
	}

	// The next evening is a new window.
	c3 := NewNightController(m)
	c3.tick(context.Background(), time.Date(2025, 1, 11, 23, 30, 0, 0, time.UTC))
	if c3.used != 0 {
		t.Fatalf("new window did not reset usage: %d", c3.used)
	}
	if win, used := loadNightUsage(c3.usagePath); win != "2025-01-11" || used != 0 {
		t.Fatalf("persisted after reset: %q %d", win, used)
	}
}

// A night job the user starts by hand after the cap is spent is paused again.
func TestNightCapPausesManuallyStartedNightJob(t *testing.T) {
	p := nightPlan("02:00", "07:00")
	p.CapGB = 1
	m := newNightManager(t, p)
	a, _ := runningJob(m, "a", true)
	c := NewNightController(m)
	c.tick(context.Background(), nightIn)
	a.SetDownloaded(nightCapBytes(1) + 1)
	c.tick(context.Background(), nightIn.Add(time.Minute))

	manual, cancelled := runningJob(m, "manual", true) // user hit Start
	c.tick(context.Background(), nightIn.Add(2*time.Minute))
	if !cancelled.Load() || !manual.GetNightPaused() {
		t.Fatalf("manual night start after cap not paused (cancelled=%v)", cancelled.Load())
	}
	if !m.NightLimitReached(manual) {
		t.Fatalf("paused job should report the night limit")
	}
	if code := m.NightStatusCode(manual); code != ErrCodeNightLimit {
		t.Fatalf("code = %q", code)
	}
}

func TestNightTickShortensNearCap(t *testing.T) {
	p := nightPlan("02:00", "07:00")
	p.CapGB = 1
	m := newNightManager(t, p)
	j, _ := runningJob(m, "n", true)
	c := NewNightController(m)
	c.tick(context.Background(), nightIn)
	if c.nextInterval() != nightTickInterval {
		t.Fatalf("far from cap should use the normal tick")
	}
	j.SetDownloaded(nightCapBytes(1) * 85 / 100)
	c.tick(context.Background(), nightIn.Add(time.Minute))
	if c.nextInterval() != nightNearCapInterval {
		t.Fatalf("near the cap should tick fast, got %v", c.nextInterval())
	}
}

// Quitting parks running night jobs for the next window, whether or not
// resume-on-launch will run.
func TestGracefulShutdownParksRunningNightJob(t *testing.T) {
	m := newNightManager(t, nightPlan("02:00", "07:00"))
	j, _ := runningJob(m, "n", true)
	m.GracefulShutdown(context.Background())
	if !j.GetNightPaused() {
		t.Fatalf("running night job not parked at quit")
	}
}
