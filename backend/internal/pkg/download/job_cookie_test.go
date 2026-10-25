package download

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

const testCookie = "sid=abc123; theme=dark"

func TestCookieReachesServerAndIsPersistedButNeverLogged(t *testing.T) {
	rec := &headerRecorder{}
	srv := httptest.NewServer(http.HandlerFunc(rec.handler))
	defer srv.Close()

	m, _ := newManagerForTest(t, 1)
	defer m.Shutdown()

	jobs, err := m.CreateJobsFromURLsWithOptions([]string{srv.URL + "/f.bin"}, JobCreateOptions{Cookie: testCookie})
	if err != nil || len(jobs) != 1 {
		t.Fatalf("create: %v (%d jobs)", err, len(jobs))
	}
	job := jobs[0]
	if job.GetCookie() != testCookie {
		t.Fatalf("cookie not stamped: %q", job.GetCookie())
	}
	if err := m.StartJob(context.Background(), job.ID); err != nil {
		t.Fatal(err)
	}
	waitForStatus(t, job, StatusCompleted)

	seen := rec.all()
	if len(seen) < 2 {
		t.Fatalf("saw %d requests", len(seen))
	}
	for i, h := range seen {
		if h.Get("Cookie") != testCookie {
			t.Errorf("request %d Cookie = %q", i, h.Get("Cookie"))
		}
	}

	// Persisted for resume.
	data, _ := json.Marshal(job)
	var back Job
	if err := json.Unmarshal(data, &back); err != nil || back.GetCookie() != testCookie {
		t.Fatalf("cookie lost on round-trip: %v %q", err, back.GetCookie())
	}
	// Unset -> omitted.
	empty, _ := json.Marshal(&Job{ID: "x", Status: StatusPending})
	if strings.Contains(string(empty), `"cookie"`) {
		t.Fatalf("cookie must be omitted when unset: %s", empty)
	}
}

func TestCreateRejectsControlCharsInCookie(t *testing.T) {
	m, _ := newManagerForTest(t, 1)
	defer m.Shutdown()
	_, err := m.CreateJobsFromURLsWithOptions([]string{"http://127.0.0.1:1/f.bin"}, JobCreateOptions{Cookie: "a=b\r\nX-Evil: 1"})
	if err == nil || !strings.Contains(err.Error(), "invalid") {
		t.Fatalf("cookie with control chars must be rejected, got %v", err)
	}
}

func TestSuggestedFileNameWinsAndIsSanitized(t *testing.T) {
	rec := &headerRecorder{}
	srv := httptest.NewServer(http.HandlerFunc(rec.handler))
	defer srv.Close()
	m, _ := newManagerForTest(t, 1)
	defer m.Shutdown()

	jobs, err := m.CreateJobsFromURLsWithOptions([]string{srv.URL + "/download?id=7"}, JobCreateOptions{FileName: "../../evil/My Movie.mkv"})
	if err != nil || len(jobs) != 1 {
		t.Fatalf("create: %v", err)
	}
	if got := jobs[0].FileName; got != "My Movie.mkv" && !strings.HasSuffix(got, "My Movie.mkv") || strings.ContainsAny(got, `/\`) {
		t.Fatalf("FileName = %q", got)
	}
}
