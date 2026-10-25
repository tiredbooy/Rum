package download

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"
)

// newEndlessServer streams until the client goes away, so a download against it
// stays running until it is paused.
func newEndlessServer(t *testing.T) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/octet-stream")
		w.WriteHeader(http.StatusOK)
		body := make([]byte, 4096)
		for {
			if _, err := w.Write(body); err != nil {
				return
			}
			if f, ok := w.(http.Flusher); ok {
				f.Flush()
			}
			select {
			case <-r.Context().Done():
				return
			case <-time.After(10 * time.Millisecond):
			}
		}
	}))
}

func TestGracefulShutdownPausesPersistsAndMarksInterrupted(t *testing.T) {
	srv := newEndlessServer(t)
	defer srv.Close()

	m, _ := newManagerForTest(t, 1)

	for _, id := range []string{"run", "queued"} {
		m.mu.Lock()
		m.jobs[id] = &Job{ID: id, URL: srv.URL + "/" + id, FileName: id + ".bin",
			TotalSize: -1, Status: StatusPending, Priority: "normal"}
		m.mu.Unlock()
		if err := m.StartJob(nil, id); err != nil {
			t.Fatalf("StartJob(%s): %v", id, err)
		}
	}
	run, _ := m.GetJob("run")
	waitStatus(t, run, StatusRunning)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	m.GracefulShutdown(ctx)

	for _, id := range []string{"run", "queued"} {
		j, _ := m.GetJob(id)
		if j.GetStatus() != StatusPaused || !j.GetInterrupted() {
			t.Fatalf("%s: status=%q interrupted=%v, want paused+interrupted", id, j.GetStatus(), j.GetInterrupted())
		}
	}

	// What a restart would see: both flagged for resume.
	loaded, urls := map[string]*Job{}, map[string]string{}
	LoadJobsFromDisk(loaded, urls)
	for _, id := range []string{"run", "queued"} {
		j := loaded[id]
		if j == nil || j.Status != StatusPaused || !j.Interrupted {
			t.Fatalf("persisted %s = %+v, want paused+interrupted", id, j)
		}
	}
}

func TestGracefulShutdownLeavesUserPausedAndFinishedAlone(t *testing.T) {
	m, _ := newManagerForTest(t, 1)
	m.mu.Lock()
	m.jobs["user"] = &Job{ID: "user", URL: "http://x/u", Status: StatusPaused}
	m.jobs["done"] = &Job{ID: "done", URL: "http://x/d", Status: StatusCompleted}
	m.mu.Unlock()

	m.GracefulShutdown(context.Background())

	for _, id := range []string{"user", "done"} {
		j, _ := m.GetJob(id)
		if j.GetInterrupted() {
			t.Fatalf("%s must not be flagged interrupted", id)
		}
	}
}

func TestGracefulShutdownRespectsDeadline(t *testing.T) {
	m, _ := newManagerForTest(t, 1)

	// A download goroutine that ignores its pause and never returns.
	stuck := make(chan struct{})
	defer close(stuck)
	job := &Job{ID: "stuck", URL: "http://x/s", Status: StatusRunning}
	job.SetCancelFunc(func() {})
	m.mu.Lock()
	m.jobs["stuck"] = job
	m.mu.Unlock()
	m.runWG.Add(1)
	go func() { defer m.runWG.Done(); <-stuck }()

	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()
	start := time.Now()
	m.GracefulShutdown(ctx)
	if d := time.Since(start); d > 2*time.Second {
		t.Fatalf("GracefulShutdown took %v despite a 200ms deadline", d)
	}

	loaded, urls := map[string]*Job{}, map[string]string{}
	LoadJobsFromDisk(loaded, urls)
	if j := loaded["stuck"]; j == nil || !j.Interrupted {
		t.Fatalf("stuck job not persisted as interrupted: %+v", j)
	}
}

func TestPeriodicPersistWritesRunningJobs(t *testing.T) {
	m, _ := newManagerForTest(t, 1)
	defer m.Shutdown()
	m.mu.Lock()
	m.jobs["p"] = &Job{ID: "p", URL: "http://x/p", Status: StatusRunning}
	m.mu.Unlock()

	ctx, cancel := context.WithCancel(context.Background())
	var wg sync.WaitGroup
	wg.Add(1)
	go func() { defer wg.Done(); m.RunPeriodicPersist(ctx, 20*time.Millisecond) }()
	time.Sleep(150 * time.Millisecond)
	cancel()
	wg.Wait()

	loaded, urls := map[string]*Job{}, map[string]string{}
	LoadJobsFromDisk(loaded, urls)
	if loaded["p"] == nil {
		t.Fatal("periodic persist did not write jobs.json")
	}
}
