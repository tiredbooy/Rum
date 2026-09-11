package download

import (
	"context"
	"errors"
	"net"
	"os"
	"path/filepath"
	"testing"

	"github.com/tiredbooy/Rum/backend/internal/pkg/config"
)

// writeReconnectSettings persists a settings.json with the reconnect flag set to
// the given value, in an isolated config dir.
func writeReconnectSettings(t *testing.T, enabled bool) {
	t.Helper()
	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", dir)

	var s config.Setting
	s.OutDir = t.TempDir()
	s.MaxParallel = 1
	s.Connections = 1
	s.AutoResumeOnReconnect = enabled
	if err := s.Save(); err != nil {
		t.Fatalf("save settings: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dir, "rum", "settings.json")); err != nil {
		t.Fatalf("settings not written: %v", err)
	}
}

// newQuietManager builds a manager and stops only its DISPATCHER, leaving the
// scheduler open. A re-queued job is therefore observably enqueued (sched.Queued)
// without the test ever launching a real download against the network.
func newQuietManager(t *testing.T) *JobManager {
	t.Helper()
	m := NewJobManager(&Options{Parallel: 1})
	m.dispatchCancel()
	m.dispatchWG.Wait()
	t.Cleanup(m.Shutdown)
	return m
}

// addFailedJob registers a job in the manager already in the error state.
func addFailedJob(m *JobManager, id, url string, err error) *Job {
	job := &Job{ID: id, URL: url, Status: StatusError, Error: err}
	m.mu.Lock()
	m.jobs[id] = job
	m.mu.Unlock()
	return job
}

// A network-failed download is re-queued once its host answers again. This is
// the whole point of auto_resume_on_reconnect, which previously was read by no
// code at all.
func TestResumeControllerRequeuesWhenHostIsBack(t *testing.T) {
	writeReconnectSettings(t, true)
	m := newQuietManager(t)

	job := addFailedJob(m, "job-1", "https://example.test/file.bin",
		&net.OpError{Op: "dial", Err: errors.New("connection refused")})

	c := NewResumeController(m)
	var probed string
	c.probe = func(_ context.Context, addr string) bool {
		probed = addr
		return true // the network is back
	}
	c.tick(context.Background())

	if probed != "example.test:443" {
		t.Errorf("probed %q, want example.test:443", probed)
	}
	if got := job.GetStatus(); got != StatusPending && got != StatusRunning {
		t.Fatalf("status = %q, want the job to have been re-queued", got)
	}
	if job.GetError() != nil {
		t.Error("stale error not cleared on re-queue")
	}
}

// While the host is still unreachable the job must stay failed, untouched.
func TestResumeControllerLeavesJobAloneWhileOffline(t *testing.T) {
	writeReconnectSettings(t, true)
	m := newQuietManager(t)

	job := addFailedJob(m, "job-1", "https://example.test/file.bin",
		&net.OpError{Op: "dial", Err: errors.New("connection refused")})

	c := NewResumeController(m)
	c.probe = func(context.Context, string) bool { return false }
	c.tick(context.Background())

	if job.GetStatus() != StatusError {
		t.Fatalf("status = %q, want it to stay error while offline", job.GetStatus())
	}
}

// The switch must actually gate the behaviour.
func TestResumeControllerRespectsTheSetting(t *testing.T) {
	writeReconnectSettings(t, false)
	m := newQuietManager(t)

	job := addFailedJob(m, "job-1", "https://example.test/file.bin",
		&net.OpError{Op: "dial", Err: errors.New("connection refused")})

	c := NewResumeController(m)
	probeCalled := false
	c.probe = func(context.Context, string) bool { probeCalled = true; return true }
	c.tick(context.Background())

	if probeCalled {
		t.Error("probed the network with auto-resume-on-reconnect off")
	}
	if job.GetStatus() != StatusError {
		t.Fatalf("status = %q, want error (setting is off)", job.GetStatus())
	}
}

// A permanent failure (404, checksum mismatch) is not a connectivity problem and
// must not be silently retried forever.
func TestResumeControllerIgnoresPermanentFailures(t *testing.T) {
	writeReconnectSettings(t, true)
	m := newQuietManager(t)

	job := addFailedJob(m, "job-1", "https://example.test/file.bin",
		&httpStatusError{code: 404})

	c := NewResumeController(m)
	c.probe = func(context.Context, string) bool { return true }
	c.tick(context.Background())

	if job.GetStatus() != StatusError {
		t.Fatalf("status = %q, want error (404 is permanent)", job.GetStatus())
	}
}

// The per-job attempt budget stops a job that fails the instant it is retried
// from looping forever.
func TestResumeControllerStopsAfterMaxAttempts(t *testing.T) {
	writeReconnectSettings(t, true)
	m := newQuietManager(t)

	netErr := &net.OpError{Op: "dial", Err: errors.New("connection refused")}
	job := addFailedJob(m, "job-1", "https://example.test/file.bin", netErr)

	c := NewResumeController(m)
	c.probe = func(context.Context, string) bool { return true }

	for i := 0; i < maxReconnectAttempts+3; i++ {
		job.SetStatus(StatusError)
		job.SetError(netErr)
		c.tick(context.Background())
	}

	if got := c.attempts["job-1"]; got != maxReconnectAttempts {
		t.Fatalf("attempts = %d, want capped at %d", got, maxReconnectAttempts)
	}
}

func TestHostPortForURL(t *testing.T) {
	cases := []struct {
		in     string
		want   string
		wantOK bool
	}{
		{"https://example.test/a", "example.test:443", true},
		{"http://example.test/a", "example.test:80", true},
		{"http://example.test:8080/a", "example.test:8080", true},
		{"ftp://example.test/a", "", false},
		{"not a url", "", false},
	}
	for _, tc := range cases {
		got, ok := hostPortForURL(tc.in)
		if ok != tc.wantOK || got != tc.want {
			t.Errorf("hostPortForURL(%q) = (%q,%v), want (%q,%v)", tc.in, got, ok, tc.want, tc.wantOK)
		}
	}
}
