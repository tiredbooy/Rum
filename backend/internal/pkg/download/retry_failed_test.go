package download

import (
	"context"
	"errors"
	"fmt"
	"testing"
)

func failedWith(m *JobManager, id string, err error) *Job {
	j := addFailedJob(m, id, "http://example.invalid/"+id, nil)
	j.SetError(err)
	return j
}

func TestRetryFailedSelectionRules(t *testing.T) {
	m := newQuietManager(t)
	prev := diskFreeFn
	t.Cleanup(func() { diskFreeFn = prev })
	diskFreeFn = func(string) (int64, bool) { return 1 << 30, true }

	failedWith(m, "net", fmt.Errorf("connection reset by peer"))
	failedWith(m, "unknown", errors.New("weird"))
	failedWith(m, "expired", &httpStatusError{code: 404})
	failedWith(m, "blocked", &blockedPageError{})
	// Disk full and still short on space: free (1 GiB) < needed.
	failedWith(m, "full-short", &DiskFullError{NeededBytes: 4 << 30, FreeBytes: 1 << 20, Path: "/x"})
	// Disk full but the volume has grown enough since.
	failedWith(m, "full-ok", &DiskFullError{NeededBytes: 1 << 20, FreeBytes: 1 << 10, Path: "/x"})
	paused := &Job{ID: "paused", Status: StatusPaused}
	m.mu.Lock()
	m.jobs["paused"] = paused
	m.mu.Unlock()

	res := m.RetryFailed(context.Background(), false)

	want := map[string]string{
		"net": StatusPending, "unknown": StatusPending, "full-ok": StatusPending,
		"expired": StatusError, "blocked": StatusError, "full-short": StatusError,
		"paused": StatusPaused,
	}
	for id, st := range want {
		if got := m.jobs[id].GetStatus(); got != st {
			t.Errorf("%s: status = %s, want %s", id, got, st)
		}
	}
	if res.Retried != 3 {
		t.Errorf("Retried = %d, want 3", res.Retried)
	}
	if res.SkippedDiskFull != 1 || res.SkippedBlocked != 1 || res.SkippedExpired != 1 {
		t.Errorf("skips = %+v", res)
	}
}

func TestRetryFailedOverridesBlockedPage(t *testing.T) {
	m := newQuietManager(t)
	failedWith(m, "blocked", &blockedPageError{})

	res := m.RetryFailed(context.Background(), true)

	if res.Retried != 1 || res.SkippedBlocked != 0 {
		t.Fatalf("res = %+v", res)
	}
	if !m.jobs["blocked"].GetAllowBlockedPage() {
		t.Error("override should allow the HTML response for that run")
	}
}
