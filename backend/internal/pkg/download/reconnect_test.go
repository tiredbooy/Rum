package download

import (
	"context"
	"errors"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sync/atomic"
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
	c.probe = func(_ context.Context, target string) bool {
		probed = target
		return true // the network is back
	}
	c.tick(context.Background())

	if probed != "https://example.test/file.bin" {
		t.Errorf("probed %q, want the job URL", probed)
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

// writeProxySetting points the persisted Proxy setting at raw.
func writeProxySetting(t *testing.T, raw string) {
	t.Helper()
	var s config.Setting
	_ = s.LoadSettingMetadata()
	s.Proxy = raw
	if err := s.Save(); err != nil {
		t.Fatalf("save settings: %v", err)
	}
}

// The reconnect probe must travel through the configured proxy: the target host
// does not resolve, so the probe can only succeed via the proxy.
func TestDialProbeGoesThroughProxy(t *testing.T) {
	writeReconnectSettings(t, true)
	var hits atomic.Int32
	proxy := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits.Add(1)
		w.WriteHeader(http.StatusOK)
	}))
	defer proxy.Close()
	writeProxySetting(t, proxy.URL)

	if !dialProbe(context.Background(), "http://unresolvable.invalid/file.bin") {
		t.Fatal("probe failed; it did not go through the proxy")
	}
	if hits.Load() == 0 {
		t.Fatal("proxy saw no request")
	}

	// Settings change applies without restart: a dead proxy now means unreachable.
	proxy.Close()
	if dialProbe(context.Background(), "http://unresolvable.invalid/file.bin") {
		t.Fatal("probe succeeded with a dead proxy; setting not re-read")
	}
	writeProxySetting(t, "")
	if dialProbe(context.Background(), "http://unresolvable.invalid/file.bin") {
		t.Fatal("probe to an unresolvable host must fail without a proxy")
	}
}

func TestDialProbeDirect(t *testing.T) {
	writeReconnectSettings(t, true)
	srv := httptest.NewServer(http.NotFoundHandler()) // any response = reachable
	defer srv.Close()
	if !dialProbe(context.Background(), srv.URL+"/x") {
		t.Fatal("direct probe of a live server failed")
	}
}

// Progress since the last attempt resets the budget; zero-progress attempts do not.
func TestResumeControllerResetsAttemptsOnProgress(t *testing.T) {
	writeReconnectSettings(t, true)
	m := newQuietManager(t)
	job := addFailedJob(m, "job-1", "https://example.test/file.bin", io.ErrUnexpectedEOF)

	c := NewResumeController(m)
	c.probe = func(context.Context, string) bool { return true }

	for i := 0; i < maxReconnectAttempts*3; i++ {
		job.SetStatus(StatusError) // the attempt failed again...
		job.SetError(io.ErrUnexpectedEOF)
		job.SetDownloaded(int64(i+1) * 1024) // ...but moved bytes first
		c.tick(context.Background())
		if job.GetStatus() == StatusError {
			t.Fatalf("iteration %d: job with progress was abandoned", i)
		}
	}
}

func TestResumeControllerStopsZeroProgressLoops(t *testing.T) {
	writeReconnectSettings(t, true)
	m := newQuietManager(t)
	job := addFailedJob(m, "job-1", "https://example.test/file.bin", io.ErrUnexpectedEOF)
	job.SetDownloaded(4096)

	c := NewResumeController(m)
	probes := 0
	c.probe = func(context.Context, string) bool { probes++; return true }

	for i := 0; i < maxReconnectAttempts*3; i++ {
		job.SetStatus(StatusError)
		job.SetError(io.ErrUnexpectedEOF)
		c.tick(context.Background())
	}
	if got := job.GetStatus(); got != StatusError {
		t.Fatalf("status = %q, want error once the idle cap is hit", got)
	}
	// 1 baseline resume + maxReconnectAttempts zero-progress retries at most.
	if c.attempts["job-1"] > maxReconnectAttempts {
		t.Fatalf("attempts = %d, want <= %d", c.attempts["job-1"], maxReconnectAttempts)
	}
}
