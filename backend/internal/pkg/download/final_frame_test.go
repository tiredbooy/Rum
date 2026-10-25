package download

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

// A failed job's final progress frame must carry the machine error code so a
// card can offer the matching fix without a list refetch.
func TestFinalErrorFrameCarriesErrorCode(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "gone", http.StatusNotFound)
	}))
	defer srv.Close()

	m, _ := newManagerForTest(t, 1)
	defer m.Shutdown()
	ch := m.SubscribeAll()

	job := &Job{ID: "ff", URL: srv.URL + "/f.bin", FileName: "f.bin", TotalSize: -1, Status: StatusPending, Priority: "normal"}
	m.mu.Lock()
	m.jobs[job.ID] = job
	m.mu.Unlock()
	if err := m.StartJob(nil, job.ID); err != nil {
		t.Fatal(err)
	}

	deadline := time.After(10 * time.Second)
	for {
		select {
		case u := <-ch:
			if u.JobID != "ff" || u.Status != StatusError {
				continue
			}
			if u.Error == "" || u.ErrorCode == "" {
				t.Fatalf("final error frame lacks fields: %+v", u)
			}
			return
		case <-deadline:
			t.Fatal("no final error frame")
		}
	}
}
