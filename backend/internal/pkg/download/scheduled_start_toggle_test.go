package download

import (
	"context"
	"testing"
	"time"

	"github.com/tiredbooy/Rum/backend/internal/pkg/config"
)

// writeScheduleSettings persists a settings.json with the scheduled-start toggle
// in the given position, in an isolated config dir.
func writeScheduleSettings(t *testing.T, enabled bool) {
	t.Helper()
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())

	var s config.Setting
	s.OutDir = t.TempDir()
	s.MaxParallel = 1
	s.Connections = 1
	s.ScheduledStartEnabled = enabled
	if err := s.Save(); err != nil {
		t.Fatalf("save settings: %v", err)
	}
}

// addDueScheduledJob registers a pending job whose scheduled start time has passed.
func addDueScheduledJob(m *JobManager, id string) *Job {
	job := &Job{ID: id, URL: "https://example.test/f.bin", Status: StatusPending}
	job.SetStartAt(time.Now().Add(-time.Hour))
	m.mu.Lock()
	m.jobs[id] = job
	m.mu.Unlock()
	return job
}

// With the toggle ON a due scheduled job is started by the controller.
func TestScheduledStartEnabledStartsDueJobs(t *testing.T) {
	writeScheduleSettings(t, true)
	m := newQuietManager(t)
	job := addDueScheduledJob(m, "job-due")

	c := NewScheduleController(m, NewSpeedGovernor(0), config.Setting{})
	c.tick(context.Background())

	// The dispatcher is stopped in this harness, so a started job stays queued —
	// which is exactly the observable "the controller started it" signal.
	if m.sched.Queued() != 1 {
		t.Fatalf("queued = %d, want the due scheduled job enqueued with the toggle on", m.sched.Queued())
	}
	_ = job
}

// With the toggle OFF the job stays pending for the user to start by hand.
// Before this was wired, the setting was persisted and rendered but branched
// nothing: due jobs started either way.
func TestScheduledStartDisabledLeavesJobPending(t *testing.T) {
	writeScheduleSettings(t, false)
	m := newQuietManager(t)
	job := addDueScheduledJob(m, "job-due")

	c := NewScheduleController(m, NewSpeedGovernor(0), config.Setting{})
	c.tick(context.Background())

	if m.sched.Queued() != 0 {
		t.Fatalf("queued = %d, want 0 — a due job must not start with the toggle off", m.sched.Queued())
	}
	if job.GetStatus() != StatusPending {
		t.Fatalf("status = %q, want pending", job.GetStatus())
	}
}

// Bandwidth windows are deliberately independent of the scheduled-start toggle.
func TestBandwidthWindowAppliesWithScheduledStartOff(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	var s config.Setting
	s.OutDir = t.TempDir()
	s.MaxParallel = 1
	s.Connections = 1
	s.ScheduledStartEnabled = false
	s.SpeedLimitKB = 0
	s.BandwidthSchedule = []config.SpeedRule{{StartHour: 0, EndHour: 0, LimitKBps: 42}}
	if err := s.Save(); err != nil {
		t.Fatalf("save settings: %v", err)
	}

	m := newQuietManager(t)
	gov := NewSpeedGovernor(0)
	c := NewScheduleController(m, gov, config.Setting{})
	c.tick(context.Background())

	if gov.LimitKBps() != 42 {
		t.Fatalf("governor limit = %d, want the all-day window's 42", gov.LimitKBps())
	}
}
