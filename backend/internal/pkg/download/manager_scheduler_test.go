package download

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"runtime"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/tiredbooy/Rum/backend/internal/pkg/config"
)

// slowServer streams a small body slowly so several jobs overlap in time,
// letting us observe the concurrency gate. It also records peak in-flight
// requests so we can assert the scheduler never exceeds the configured max.
func newSlowServer(t *testing.T, peak *int32) *httptest.Server {
	t.Helper()
	var inFlight int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		cur := atomic.AddInt32(&inFlight, 1)
		defer atomic.AddInt32(&inFlight, -1)
		for {
			prev := atomic.LoadInt32(peak)
			if cur <= prev || atomic.CompareAndSwapInt32(peak, prev, cur) {
				break
			}
		}
		w.Header().Set("Content-Type", "application/octet-stream")
		w.WriteHeader(http.StatusOK)
		// Write a little, sleep, write a little: keeps the connection busy long
		// enough for downloads to overlap, while bailing out promptly if the
		// client/server goes away (so no test goroutine lingers).
		body := make([]byte, 4096)
		for i := 0; i < 5; i++ {
			w.Write(body)
			if f, ok := w.(http.Flusher); ok {
				f.Flush()
			}
			select {
			case <-r.Context().Done():
				return
			case <-time.After(30 * time.Millisecond):
			}
		}
	}))
	return srv
}

func newManagerForTest(t *testing.T, parallel int) (*JobManager, string) {
	t.Helper()
	// Isolate jobs.json / settings.json under a temp config dir.
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	outDir := t.TempDir()
	// Persist Silent=true (so completionOperations, which re-reads settings from
	// disk, does not fire beeep/Notify and block in a headless env) and point the
	// download dir at a temp dir so tests never touch the real ~/Downloads/Rum.
	persistTestSettings(t, outDir)
	opt := &Options{
		Parallel:    parallel,
		Connections: 1, // single-stream; small bodies, no segmenting
		MaxRetries:  0,
		Silent:      true,
		Out:         outDir,
		Downloader:  NewDownloader("test-agent", ""),
	}
	m := NewJobManager(opt)
	return m, outDir
}

// persistTestSettings writes a settings.json (under the test's isolated config
// dir) with Silent enabled and OutDir pinned to a temp dir, so post-completion
// notifications are suppressed and downloads never land in the real Downloads
// folder.
func persistTestSettings(t *testing.T, outDir string) {
	t.Helper()
	var s config.Setting
	if err := s.LoadSettingMetadata(); err != nil {
		t.Logf("load settings for test: %v", err)
	}
	s.Silent = true
	s.OutDir = outDir
	if err := s.Save(); err != nil {
		t.Fatalf("persist test settings: %v", err)
	}
}

func TestSchedulerGatesConcurrency(t *testing.T) {
	var peak int32
	srv := newSlowServer(t, &peak)
	defer srv.Close()

	m, _ := newManagerForTest(t, 1) // at most 1 concurrent download
	defer m.Shutdown()

	// Inject 5 jobs directly (bypass HEAD) with varied priorities.
	const n = 5
	ids := make([]string, 0, n)
	for i := 0; i < n; i++ {
		job := &Job{
			ID:           fmt.Sprintf("job-%d", i),
			URL:          srv.URL + fmt.Sprintf("/f%d.bin", i),
			FileName:     fmt.Sprintf("f%d.bin", i),
			TotalSize:    -1,
			SupportRange: false,
			Status:       StatusPending,
			Priority:     "normal",
		}
		m.mu.Lock()
		m.jobs[job.ID] = job
		m.mu.Unlock()
		ids = append(ids, job.ID)
	}

	for _, id := range ids {
		if err := m.StartJob(nil, id); err != nil {
			t.Fatalf("StartJob(%s): %v", id, err)
		}
	}

	// Poll the scheduler's running count; it must never exceed max=1.
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		if r := m.sched.Running(); r > 1 {
			t.Fatalf("scheduler ran %d jobs concurrently, want <= 1", r)
		}
		done := 0
		m.mu.RLock()
		for _, id := range ids {
			if m.jobs[id].GetStatus() == StatusCompleted {
				done++
			}
		}
		m.mu.RUnlock()
		if done == n {
			break
		}
		time.Sleep(15 * time.Millisecond)
	}

	if peak > 1 {
		t.Fatalf("server saw %d concurrent requests, want <= 1", peak)
	}
}

func TestSchedulerPriorityOrdering(t *testing.T) {
	var peak int32
	srv := newSlowServer(t, &peak)
	defer srv.Close()

	m, _ := newManagerForTest(t, 1)
	defer m.Shutdown()

	// Record completion order. Filter to this test's own ids so a late completion
	// from another test's manager (the hook is process-global) can't contaminate.
	mine := map[string]bool{"low": true, "high": true, "blocker": true}
	var mu sync.Mutex
	var order []string
	origHook := setTestCompletionHook(func(id string) {
		if !mine[id] {
			return
		}
		mu.Lock()
		order = append(order, id)
		mu.Unlock()
	})
	defer setTestCompletionHook(origHook)

	// Queue a low-priority job first, then a high-priority one. With max=1 and the
	// scheduler picking highest priority, "high" should finish before "low" even
	// though "low" was submitted earlier — provided they are submitted before the
	// dispatcher grants a slot. To make the race deterministic we submit both
	// while the dispatcher is momentarily idle by submitting back to back.
	low := &Job{ID: "low", URL: srv.URL + "/low.bin", FileName: "low.bin", TotalSize: -1, Status: StatusPending, Priority: "low"}
	high := &Job{ID: "high", URL: srv.URL + "/high.bin", FileName: "high.bin", TotalSize: -1, Status: StatusPending, Priority: "high"}
	blocker := &Job{ID: "blocker", URL: srv.URL + "/b.bin", FileName: "b.bin", TotalSize: -1, Status: StatusPending, Priority: "normal"}

	m.mu.Lock()
	m.jobs[low.ID] = low
	m.jobs[high.ID] = high
	m.jobs[blocker.ID] = blocker
	m.mu.Unlock()

	// Start a blocker first to occupy the single slot, then submit low + high so
	// they queue together. When the blocker finishes, the scheduler grants high
	// before low.
	if err := m.StartJob(nil, blocker.ID); err != nil {
		t.Fatal(err)
	}
	// Deterministically wait until the dispatcher has actually started the blocker
	// (occupying the single slot) before queueing low/high, so they are guaranteed
	// to wait together for the scheduler to pick the next-highest priority.
	waitDeadline := time.Now().Add(3 * time.Second)
	for m.sched.Running() < 1 && time.Now().Before(waitDeadline) {
		time.Sleep(2 * time.Millisecond)
	}
	if m.sched.Running() < 1 {
		t.Fatal("blocker did not start occupying the slot")
	}
	if err := m.StartJob(nil, low.ID); err != nil {
		t.Fatal(err)
	}
	if err := m.StartJob(nil, high.ID); err != nil {
		t.Fatal(err)
	}

	// Wait for all three to complete.
	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		mu.Lock()
		n := len(order)
		mu.Unlock()
		if n == 3 {
			break
		}
		time.Sleep(15 * time.Millisecond)
	}

	mu.Lock()
	defer mu.Unlock()
	if len(order) != 3 {
		t.Fatalf("expected 3 completions, got %d (%v)", len(order), order)
	}
	// Find positions of high and low.
	posHigh, posLow := -1, -1
	for i, id := range order {
		if id == "high" {
			posHigh = i
		}
		if id == "low" {
			posLow = i
		}
	}
	if posHigh > posLow {
		t.Fatalf("high-priority job completed after low (order=%v)", order)
	}
}

func TestDispatcherNoGoroutineLeak(t *testing.T) {
	var peak int32
	srv := newSlowServer(t, &peak)

	const n = 4
	dl := NewDownloader("test-agent", "")

	// Baseline AFTER the server exists (its accept/handler goroutines are excluded
	// from the leak budget).
	before := runtime.NumGoroutine()

	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	outDir := t.TempDir()
	persistTestSettings(t, outDir)
	opt := &Options{Parallel: 2, Connections: 1, MaxRetries: 0, Silent: true, Out: outDir, Downloader: dl}
	m := NewJobManager(opt)

	ids := make([]string, 0, n)
	for i := 0; i < n; i++ {
		job := &Job{ID: fmt.Sprintf("g%d", i), URL: srv.URL + fmt.Sprintf("/g%d.bin", i), FileName: fmt.Sprintf("g%d.bin", i), TotalSize: -1, Status: StatusPending, Priority: "normal"}
		m.mu.Lock()
		m.jobs[job.ID] = job
		m.mu.Unlock()
		ids = append(ids, job.ID)
	}
	for _, id := range ids {
		_ = m.StartJob(nil, id)
	}

	// Wait for all to finish.
	deadline := time.Now().Add(6 * time.Second)
	for time.Now().Before(deadline) {
		done := 0
		m.mu.RLock()
		for _, id := range ids {
			if m.jobs[id].GetStatus() == StatusCompleted {
				done++
			}
		}
		m.mu.RUnlock()
		if done == n {
			break
		}
		time.Sleep(15 * time.Millisecond)
	}

	// Shut down the dispatcher and release idle keep-alive connections + server so
	// only a real manager/dispatcher leak would remain.
	m.Shutdown()
	dl.Client.CloseIdleConnections()
	srv.Close()

	// Allow goroutines to wind down.
	deadline = time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		if runtime.NumGoroutine() <= before+2 {
			break
		}
		time.Sleep(20 * time.Millisecond)
	}
	after := runtime.NumGoroutine()
	if after > before+3 { // small slack for test-runtime/transport goroutines
		t.Fatalf("goroutine leak: before=%d after=%d", before, after)
	}
}
