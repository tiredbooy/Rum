package download

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	filesystem "github.com/tiredbooy/Rum/backend/internal/pkg/file-system"
)

// reload simulates an app restart: it persists the given jobs as the previous
// session left them, then loads jobs.json into a clean job set.
func reload(t *testing.T, m *JobManager, prev []*Job) {
	t.Helper()
	if err := PersistJobs(prev); err != nil {
		t.Fatalf("persist: %v", err)
	}
	m.mu.Lock()
	m.jobs = map[string]*Job{}
	m.urls = map[string]string{}
	m.loadFromDisk()
	m.mu.Unlock()
}

func TestLoadKeepsErrorJobsWithMessage(t *testing.T) {
	m, _ := newManagerForTest(t, 1)
	defer m.Shutdown()

	reload(t, m, []*Job{
		{ID: "e", URL: "http://x/e", Status: StatusError, Error: errors.New("no space left on device")},
	})

	j, ok := m.GetJob("e")
	if !ok {
		t.Fatal("error job missing after load")
	}
	if j.GetStatus() != StatusError {
		t.Fatalf("error job status = %q, want error", j.GetStatus())
	}
	if j.GetError() == nil || j.GetError().Error() != "no space left on device" {
		t.Fatalf("error message lost: %v", j.GetError())
	}
	if j.GetInterrupted() {
		t.Fatal("error job must not be flagged interrupted")
	}
}

func TestLoadFlagsRunningAndPendingAsInterrupted(t *testing.T) {
	m, _ := newManagerForTest(t, 1)
	defer m.Shutdown()

	reload(t, m, []*Job{
		{ID: "run", URL: "http://x/run", Status: StatusRunning},
		{ID: "pend", URL: "http://x/pend", Status: StatusPending},
		{ID: "user", URL: "http://x/user", Status: StatusPaused},
		{ID: "done", URL: "http://x/done", Status: StatusCompleted},
		{ID: "later", URL: "http://x/later", Status: StatusPending, StartAt: time.Now().Add(time.Hour)},
	})

	for id, want := range map[string]struct {
		status      string
		interrupted bool
	}{
		"run":   {StatusPaused, true},
		"pend":  {StatusPaused, true},
		"user":  {StatusPaused, false},
		"done":  {StatusCompleted, false},
		"later": {StatusPending, false}, // left for the schedule controller
	} {
		j, _ := m.GetJob(id)
		if j.GetStatus() != want.status || j.GetInterrupted() != want.interrupted {
			t.Errorf("%s: status=%q interrupted=%v, want %q/%v", id, j.GetStatus(), j.GetInterrupted(), want.status, want.interrupted)
		}
	}
}

func TestOldJobsFileWithoutFlagLoads(t *testing.T) {
	m, _ := newManagerForTest(t, 1)
	defer m.Shutdown()

	old := `[{"id":"a","url":"http://x/a","status":"paused"},{"id":"b","url":"http://x/b","status":"error","error":"boom"}]`
	if err := filesystem.WriteMetadataFile("jobs.json", json.RawMessage(old)); err != nil {
		t.Fatal(err)
	}
	m.mu.Lock()
	m.jobs = map[string]*Job{}
	m.urls = map[string]string{}
	m.loadFromDisk()
	m.mu.Unlock()

	a, _ := m.GetJob("a")
	b, _ := m.GetJob("b")
	if a == nil || b == nil {
		t.Fatal("old-format jobs did not load")
	}
	if a.GetInterrupted() || a.GetStatus() != StatusPaused {
		t.Errorf("old paused job must stay a plain user pause: %q/%v", a.GetStatus(), a.GetInterrupted())
	}
	if b.GetStatus() != StatusError || b.GetError() == nil {
		t.Errorf("old error job lost its state: %q %v", b.GetStatus(), b.GetError())
	}
}

func TestInterruptedFlagRoundTripsAndOmittedWhenFalse(t *testing.T) {
	data, _ := (&Job{ID: "1", Status: StatusPaused}).MarshalJSON()
	if bytes.Contains(data, []byte("interrupted")) {
		t.Fatalf("flag must be omitted when false: %s", data)
	}
	data, _ = (&Job{ID: "1", Status: StatusPaused, Interrupted: true}).MarshalJSON()
	var back Job
	if err := back.UnmarshalJSON(data); err != nil || !back.Interrupted {
		t.Fatalf("flag did not round-trip: %v %s", err, data)
	}
}

func TestResumeInterruptedJobsResumesOnlyFlagged(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/octet-stream")
		w.Write(make([]byte, 2048))
	}))
	defer srv.Close()

	m, _ := newManagerForTest(t, 2)
	defer m.Shutdown()

	reload(t, m, []*Job{
		{ID: "crashed", URL: srv.URL + "/a.bin", FileName: "a.bin", TotalSize: -1, Status: StatusRunning},
		{ID: "userpaused", URL: srv.URL + "/b.bin", FileName: "b.bin", TotalSize: -1, Status: StatusPaused},
	})

	if n := m.ResumeInterruptedJobs(context.Background()); n != 1 {
		t.Fatalf("resumed %d, want 1", n)
	}

	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		if j, _ := m.GetJob("crashed"); j.GetStatus() == StatusCompleted {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	if j, _ := m.GetJob("crashed"); j.GetStatus() != StatusCompleted || j.GetInterrupted() {
		t.Fatalf("crashed job: status=%q interrupted=%v, want completed/cleared", j.GetStatus(), j.GetInterrupted())
	}
	if j, _ := m.GetJob("userpaused"); j.GetStatus() != StatusPaused {
		t.Fatalf("user-paused job was touched: %q", j.GetStatus())
	}
}

// A kill -9 right after a job starts must still leave it recoverable: the
// transition into running is persisted (debounced) without waiting for a finish.
func TestRunningTransitionIsPersistedForCrashRecovery(t *testing.T) {
	release := make(chan struct{})
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/octet-stream")
		w.WriteHeader(http.StatusOK)
		w.(http.Flusher).Flush()
		select {
		case <-release:
		case <-r.Context().Done():
		}
	}))
	defer srv.Close()
	defer close(release)

	m, _ := newManagerForTest(t, 1)
	defer m.Shutdown()

	j := &Job{ID: "live", URL: srv.URL + "/f.bin", FileName: "f.bin", TotalSize: -1, Status: StatusPending}
	m.mu.Lock()
	m.jobs[j.ID] = j
	m.mu.Unlock()
	if err := m.StartJob(context.Background(), j.ID); err != nil {
		t.Fatal(err)
	}

	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		for _, p := range readJobsFile() {
			if p.ID == "live" && (p.Status == StatusRunning) {
				return
			}
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatal("running state never reached jobs.json")
}
