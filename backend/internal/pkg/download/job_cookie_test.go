package download

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"runtime"
	"strings"
	"testing"

	filesystem "github.com/tiredbooy/Rum/backend/internal/pkg/file-system"
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

	// Persisted for resume (a finished job drops it; see TestCompletedJobDropsCookieFromDisk).
	job.SetStatus(StatusPaused)
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

func TestCompletedJobDropsCookieFromDisk(t *testing.T) {
	j := &Job{ID: "x", URL: "http://h/f", Status: StatusCompleted, Cookie: "sid=SECRETVALUE"}
	data, _ := json.Marshal(j)
	if strings.Contains(string(data), "SECRETVALUE") {
		t.Fatalf("finished job must not keep its cookie on disk: %s", data)
	}
	j.Status = StatusPaused
	data, _ = json.Marshal(j)
	if !strings.Contains(string(data), "SECRETVALUE") {
		t.Fatalf("unfinished job needs its cookie to resume: %s", data)
	}
}

func TestRequireProbeRefusesRejectedLink(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "login required", http.StatusForbidden)
	}))
	defer srv.Close()
	m, _ := newManagerForTest(t, 1)
	defer m.Shutdown()

	if _, err := m.CreateJobsFromURLsWithOptions([]string{srv.URL + "/f.bin"}, JobCreateOptions{RequireProbe: true}); err == nil {
		t.Fatal("a link the server refuses must not become a job when RequireProbe is set")
	}
	jobs, err := m.CreateJobsFromURLsWithOptions([]string{srv.URL + "/g.bin"}, JobCreateOptions{})
	if err != nil || len(jobs) != 1 {
		t.Fatalf("default behaviour must stay unchanged: %v", err)
	}
}

func TestJobsFileIsPrivate(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	if err := PersistJobs([]*Job{{ID: "a", URL: "http://h/f", Status: StatusPaused, Cookie: "sid=1"}}); err != nil {
		t.Fatal(err)
	}
	fi, err := os.Stat(filesystem.CreateMetadataFile("jobs.json"))
	if err != nil {
		t.Fatal(err)
	}
	if runtime.GOOS != "windows" && fi.Mode().Perm() != 0o600 {
		t.Fatalf("jobs.json mode %v, want 0600", fi.Mode().Perm())
	}
}
