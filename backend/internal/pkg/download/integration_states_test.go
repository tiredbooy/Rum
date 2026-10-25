package download

import (
	"context"
	"encoding/json"
	"errors"
	"net"
	"testing"
)

// reloaded simulates a restart: the job is written to jobs.json form and read
// back, so only persisted fields (error text, code, flags) survive.
func reloaded(t *testing.T, j *Job) *Job {
	t.Helper()
	b, err := json.Marshal(j)
	if err != nil {
		t.Fatal(err)
	}
	var back Job
	if err := json.Unmarshal(b, &back); err != nil {
		t.Fatal(err)
	}
	return &back
}

func failedJob(err error) *Job {
	j := &Job{ID: "j", URL: "https://example.test/f.bin", Status: StatusError}
	j.SetError(err)
	return j
}

// The reconnect controller must leave disk_full, blocked_page and link_expired
// jobs alone, both live and after a restart (when only the code survives), but
// must resume a persisted network failure.
func TestReconnectEligibilityAcrossStatesAndRestarts(t *testing.T) {
	c := NewResumeController(nil)
	cases := []struct {
		name string
		err  error
		want bool
	}{
		{"disk full typed", &DiskFullError{NeededBytes: 10, FreeBytes: 1, Path: "/x"}, false},
		{"blocked page", &blockedPageError{}, false},
		{"link expired", &httpStatusError{code: 410}, false},
		{"network down", &net.OpError{Op: "dial", Err: errors.New("connection refused")}, true},
		{"stalled after retries", errStalled, true},
	}
	for _, tc := range cases {
		live := failedJob(tc.err)
		if got := c.eligible(live); got != tc.want {
			t.Errorf("%s (live): eligible=%v want %v", tc.name, got, tc.want)
		}
		if got := c.eligible(reloaded(t, live)); got != tc.want {
			code, _ := reloaded(t, live).GetErrorCode()
			t.Errorf("%s (after restart, code %q): eligible=%v want %v", tc.name, code, got, tc.want)
		}
	}
}

// A stalled retry in flight carries the transient code but must not persist it;
// a job that FAILED stalled keeps it.
func TestStalledCodePersistence(t *testing.T) {
	j := &Job{ID: "s", Status: StatusRunning}
	j.MarkStalled()
	if code, _ := reloaded(t, j).GetErrorCode(); code != "" {
		t.Errorf("transient stalled code persisted: %q", code)
	}
	j.ClearStalled()
	if j.GetStallCode() != "" {
		t.Error("ClearStalled left the code")
	}
	failed := failedJob(errStalled)
	if code, _ := reloaded(t, failed).GetErrorCode(); code != ErrorCodeStalled {
		t.Errorf("failed-stalled code = %q after restart", code)
	}
	// ClearStalled must not erase a real error code.
	real := failedJob(&httpStatusError{code: 503})
	real.ClearStalled()
	if code, _ := real.GetErrorCode(); code != ErrCodeServerError {
		t.Errorf("ClearStalled clobbered %q", code)
	}
}

// Launch resume only touches paused+interrupted jobs: a disk_full or blocked
// error job (not interrupted) stays as it is.
func TestLaunchResumeIgnoresErrorStates(t *testing.T) {
	writeReconnectSettings(t, true)
	m := newQuietManager(t)
	d := addFailedJob(m, "d", "https://example.test/d", &DiskFullError{NeededBytes: 5})
	b := addFailedJob(m, "b", "https://example.test/b", &blockedPageError{})
	if n := m.ResumeInterruptedJobs(context.Background()); n != 0 {
		t.Fatalf("resumed %d jobs, want 0", n)
	}
	if d.GetStatus() != StatusError || b.GetStatus() != StatusError {
		t.Fatalf("states changed: %s %s", d.GetStatus(), b.GetStatus())
	}
}
