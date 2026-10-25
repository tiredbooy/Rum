package download

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

// headerRecorder serves a small file and records the request headers of the
// requests it receives (HEAD probe and GET alike).
type headerRecorder struct {
	mu   sync.Mutex
	seen []http.Header
}

func (h *headerRecorder) handler(w http.ResponseWriter, r *http.Request) {
	h.mu.Lock()
	h.seen = append(h.seen, r.Header.Clone())
	h.mu.Unlock()
	w.Header().Set("Content-Type", "application/octet-stream")
	w.Header().Set("Accept-Ranges", "bytes")
	http.ServeContent(w, r, "f.bin", time.Time{}, newReadSeeker(makePayload(2048)))
}

func (h *headerRecorder) all() []http.Header {
	h.mu.Lock()
	defer h.mu.Unlock()
	return append([]http.Header(nil), h.seen...)
}

func waitForStatus(t *testing.T, j *Job, want string) {
	t.Helper()
	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		if j.GetStatus() == want {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("job status = %q (err %v), want %q", j.GetStatus(), j.GetError(), want)
}

func TestJobOptionsRoundTrip(t *testing.T) {
	j := &Job{
		ID: "o1", URL: "http://x/y", Status: StatusPending,
		Referer: "http://ref/", UserAgent: "UA/1", SpeedLimit: 128, MaxRetries: 7,
		DestPath: "/data/dl", ErrorCode: "link_expired",
	}
	data, err := json.Marshal(j)
	if err != nil {
		t.Fatal(err)
	}
	var back Job
	if err := json.Unmarshal(data, &back); err != nil {
		t.Fatal(err)
	}
	if back.Referer != "http://ref/" || back.UserAgent != "UA/1" || back.SpeedLimit != 128 ||
		back.MaxRetries != 7 || back.DestPath != "/data/dl" || back.ErrorCode != "link_expired" {
		t.Fatalf("per-job options lost on round-trip: %+v", &back)
	}

	empty, _ := json.Marshal(&Job{ID: "o2", Status: StatusPending})
	for _, k := range []string{"referer", "user_agent", "speed_limit", "max_retries", "dest_path", "error_code"} {
		if strings.Contains(string(empty), `"`+k+`"`) {
			t.Errorf("%s must be omitted when unset: %s", k, empty)
		}
	}
}

func TestApplyJobOptionsOverlay(t *testing.T) {
	m, outDir := newManagerForTest(t, 1)
	defer m.Shutdown()

	global := m.effectiveOptsForJob(&Job{})
	if global.Out != outDir {
		t.Fatalf("precondition: Out = %q", global.Out)
	}

	dest := t.TempDir()
	job := &Job{ID: "ov", MaxRetries: 9, SpeedLimit: 50, DestPath: dest}
	opt := m.effectiveOptsForJob(job)
	if opt.MaxRetries != 9 {
		t.Errorf("MaxRetries = %d, want 9", opt.MaxRetries)
	}
	if opt.SpeedLimit != 50 || opt.JobSpeedLimit != 50 {
		t.Errorf("SpeedLimit/JobSpeedLimit = %d/%d, want 50/50", opt.SpeedLimit, opt.JobSpeedLimit)
	}
	if opt.Out != dest {
		t.Errorf("Out = %q, want %q", opt.Out, dest)
	}

	// Unset fields keep the global values.
	plain := m.effectiveOptsForJob(&Job{ID: "pl"})
	if plain.Out != outDir || plain.JobSpeedLimit != 0 || plain.Downloader != m.opt.Downloader {
		t.Errorf("empty job must not change global opts: %+v", plain)
	}
}

func TestPerJobRefererAndUserAgentReachServer(t *testing.T) {
	rec := &headerRecorder{}
	srv := httptest.NewServer(http.HandlerFunc(rec.handler))
	defer srv.Close()

	m, outDir := newManagerForTest(t, 1)
	defer m.Shutdown()

	jobs, err := m.CreateJobsFromURLsWithOptions([]string{srv.URL + "/f.bin"}, JobCreateOptions{
		Referer: "https://origin.example/page", UserAgent: "RumTest/9", MaxRetries: 4, SpeedLimit: 900,
	})
	if err != nil || len(jobs) != 1 {
		t.Fatalf("create: %v (%d jobs)", err, len(jobs))
	}
	job := jobs[0]
	if job.GetReferer() != "https://origin.example/page" || job.GetUserAgent() != "RumTest/9" ||
		job.GetMaxRetries() != 4 || job.GetSpeedLimit() != 900 {
		t.Fatalf("options not stamped on job: %+v", job)
	}

	if err := m.StartJob(context.Background(), job.ID); err != nil {
		t.Fatal(err)
	}
	waitForStatus(t, job, StatusCompleted)

	seen := rec.all()
	if len(seen) < 2 {
		t.Fatalf("expected the probe and the download to hit the server, saw %d", len(seen))
	}
	for i, h := range seen {
		if h.Get("User-Agent") != "RumTest/9" {
			t.Errorf("request %d User-Agent = %q", i, h.Get("User-Agent"))
		}
		if h.Get("Referer") != "https://origin.example/page" {
			t.Errorf("request %d Referer = %q", i, h.Get("Referer"))
		}
	}
	if _, err := os.Stat(filepath.Join(outDir, "f.bin")); err != nil {
		// default category folders may apply; just ensure something landed in outDir
		found := false
		_ = filepath.Walk(outDir, func(p string, info os.FileInfo, _ error) error {
			if info != nil && !info.IsDir() && info.Name() == "f.bin" {
				found = true
			}
			return nil
		})
		if !found {
			t.Fatalf("downloaded file not found under %s", outDir)
		}
	}

	// Persisted: a reload keeps the per-job options.
	m.saveToDisk()
	m.mu.Lock()
	m.jobs = map[string]*Job{}
	m.urls = map[string]string{}
	m.loadFromDisk()
	m.mu.Unlock()
	back, ok := m.GetJob(job.ID)
	if !ok || back.GetReferer() != "https://origin.example/page" || back.GetUserAgent() != "RumTest/9" ||
		back.GetMaxRetries() != 4 || back.GetSpeedLimit() != 900 {
		t.Fatalf("options not persisted: %+v", back)
	}
}

func TestPerJobDestPathReceivesFile(t *testing.T) {
	rec := &headerRecorder{}
	srv := httptest.NewServer(http.HandlerFunc(rec.handler))
	defer srv.Close()

	m, outDir := newManagerForTest(t, 1)
	defer m.Shutdown()

	jobs, err := m.CreateJobsFromURLsWithOptions([]string{srv.URL + "/f.bin"}, JobCreateOptions{DestPath: "custom/sub"})
	if err != nil || len(jobs) != 1 {
		t.Fatalf("create: %v", err)
	}
	job := jobs[0]
	want := filepath.Join(outDir, "custom", "sub")
	if job.GetDestPath() != want {
		t.Fatalf("DestPath = %q, want resolved %q", job.GetDestPath(), want)
	}
	if err := m.StartJob(context.Background(), job.ID); err != nil {
		t.Fatal(err)
	}
	waitForStatus(t, job, StatusCompleted)
	if _, err := os.Stat(filepath.Join(want, "f.bin")); err != nil {
		t.Fatalf("file not in dest path: %v", err)
	}
}

func TestCreateRejectsEscapingDestPath(t *testing.T) {
	m, _ := newManagerForTest(t, 1)
	defer m.Shutdown()
	_, err := m.CreateJobsFromURLsWithOptions([]string{"http://127.0.0.1:1/f.bin"}, JobCreateOptions{DestPath: "../../etc"})
	if err == nil || !errors.Is(err, ErrInvalidURL) && !strings.Contains(err.Error(), "dest") {
		t.Fatalf("escaping dest path must be rejected, got %v", err)
	}
}

func TestResolveDestPath(t *testing.T) {
	base := filepath.Join(t.TempDir(), "Downloads")
	abs := filepath.Join(t.TempDir(), "elsewhere")
	file := filepath.Join(t.TempDir(), "afile")
	if err := os.WriteFile(file, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}

	cases := []struct {
		name, in, want string
		wantErr        bool
	}{
		{"empty keeps global", "", "", false},
		{"relative under base", "movies/2024", filepath.Join(base, "movies", "2024"), false},
		{"absolute kept", abs, abs, false},
		{"relative traversal", "../../etc/cron.d", "", true},
		{"embedded traversal", "a/../../b", "", true},
		{"absolute traversal", abs + "/../x", "", true},
		{"nul byte", "a\x00b", "", true},
		{"existing file not dir", file, "", true},
	}
	for _, c := range cases {
		got, err := ResolveDestPath(base, c.in)
		if (err != nil) != c.wantErr {
			t.Errorf("%s: err = %v, wantErr %v", c.name, err, c.wantErr)
			continue
		}
		if !c.wantErr && got != c.want {
			t.Errorf("%s: got %q, want %q", c.name, got, c.want)
		}
	}
}

func TestLinkExpiredCode(t *testing.T) {
	for code, want := range map[int]string{
		403: ErrCodeLinkExpired, 404: ErrCodeLinkExpired, 410: ErrCodeLinkExpired,
		500: "", 429: "", 401: "",
	} {
		if got := linkExpiredCode(&httpStatusError{code: code}); got != want {
			t.Errorf("status %d -> %q, want %q", code, got, want)
		}
	}
	wrapped := errors.Join(errors.New("ctx"), &httpStatusError{code: 410})
	if linkExpiredCode(wrapped) != ErrCodeLinkExpired {
		t.Error("wrapped 410 must map to link_expired")
	}
	if linkExpiredCode(nil) != "" || linkExpiredCode(errors.New("boom")) != "" {
		t.Error("non-HTTP errors must not map to link_expired")
	}
}

func TestExpiredLinkSetsErrorCode(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "gone", http.StatusGone)
	}))
	defer srv.Close()

	m, _ := newManagerForTest(t, 1)
	defer m.Shutdown()
	job := &Job{ID: "gone", URL: srv.URL + "/f.bin", FileName: "f.bin", TotalSize: -1, Status: StatusPending}
	m.mu.Lock()
	m.jobs[job.ID] = job
	m.urls[job.URL] = job.ID
	m.mu.Unlock()

	if err := m.StartJob(context.Background(), job.ID); err != nil {
		t.Fatal(err)
	}
	waitForStatus(t, job, StatusError)
	if got, _ := job.GetErrorCode(); got != ErrCodeLinkExpired {
		t.Fatalf("error code = %q, want link_expired", got)
	}
}

func TestRedactURL(t *testing.T) {
	got := RedactURL("https://user:pw@host.example/path/f.bin?token=SECRET&x=1#frag")
	for _, leak := range []string{"SECRET", "pw", "token", "frag"} {
		if strings.Contains(got, leak) {
			t.Errorf("RedactURL leaked %q: %s", leak, got)
		}
	}
	if !strings.Contains(got, "host.example") {
		t.Errorf("RedactURL dropped the host: %s", got)
	}
}

func TestCreateRejectsControlCharsInHeaders(t *testing.T) {
	m, _ := newManagerForTest(t, 1)
	defer m.Shutdown()
	for _, o := range []JobCreateOptions{
		{Referer: "http://a/\r\nX-Evil: 1"},
		{UserAgent: "ua\nX-Evil: 1"},
	} {
		if _, err := m.CreateJobsFromURLsWithOptions([]string{"http://127.0.0.1:1/f.bin"}, o); err == nil || !strings.Contains(err.Error(), "invalid") {
			t.Fatalf("header with control chars must be rejected, got %v", err)
		}
	}
}
