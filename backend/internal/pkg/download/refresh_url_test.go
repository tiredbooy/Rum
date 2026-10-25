package download

import (
	"bytes"
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"
)

// refreshServer serves payload with the given ETag, recording Range headers of
// GET requests so tests can prove a resume (not a restart) happened.
type refreshServer struct {
	*httptest.Server
	mu     sync.Mutex
	ranges []string
}

func newRefreshServer(t *testing.T, payload []byte, etag string) *refreshServer {
	t.Helper()
	rs := &refreshServer{}
	rs.Server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet {
			rs.mu.Lock()
			rs.ranges = append(rs.ranges, r.Header.Get("Range"))
			rs.mu.Unlock()
		}
		w.Header().Set("Accept-Ranges", "bytes")
		if etag != "" {
			w.Header().Set("ETag", etag)
		}
		http.ServeContent(w, r, "f.bin", time.Time{}, newReadSeeker(payload))
	}))
	t.Cleanup(rs.Close)
	return rs
}

func (rs *refreshServer) sawRange(want string) bool {
	rs.mu.Lock()
	defer rs.mu.Unlock()
	for _, r := range rs.ranges {
		if r == want {
			return true
		}
	}
	return false
}

// expiredJobWithPartial registers an errored (expired-link) job with the first
// `have` bytes of payload on disk, as a real interrupted download leaves it.
func expiredJobWithPartial(t *testing.T, m *JobManager, _ string, oldURL string, payload []byte, have int) (*Job, string) {
	t.Helper()
	// Where the engine will look for the partial (honors the category folder).
	path := PrepareOutputPath(*m.opt, "f.bin", oldURL, "")
	if err := os.WriteFile(path, payload[:have], 0o644); err != nil {
		t.Fatal(err)
	}
	job := &Job{
		ID: "exp", URL: oldURL, FileName: "f.bin", OutputPath: path,
		TotalSize: int64(len(payload)), Downloaded: int64(have), SupportRange: true,
		Status: StatusError, Error: &httpStatusError{code: 403}, ErrorCode: ErrCodeLinkExpired,
	}
	m.mu.Lock()
	m.jobs[job.ID] = job
	m.urls[oldURL] = job.ID
	m.mu.Unlock()
	return job, path
}

func TestRefreshJobURLMatchResumesKeepingPartial(t *testing.T) {
	payload := makePayload(64 * 1024)
	fresh := newRefreshServer(t, payload, `"v1"`)

	m, outDir := newManagerForTest(t, 1)
	defer m.Shutdown()
	job, path := expiredJobWithPartial(t, m, outDir, "http://127.0.0.1:1/old.bin", payload, 16*1024)

	res, err := m.RefreshJobURL(context.Background(), job.ID, fresh.URL+"/new.bin", false)
	if err != nil {
		t.Fatalf("refresh: %v", err)
	}
	if !res.Resumed || res.Restarted {
		t.Fatalf("result = %+v, want resumed without restart", res)
	}
	if job.GetURL() != fresh.URL+"/new.bin" {
		t.Fatalf("URL not swapped: %q", job.GetURL())
	}
	if code, _ := job.GetErrorCode(); job.GetError() != nil || code != "" {
		t.Fatalf("error not cleared: %v / %q", job.GetError(), code)
	}
	if id, ok := m.GetJobIDByURL(fresh.URL + "/new.bin"); !ok || id != job.ID {
		t.Fatal("url index not updated")
	}
	if _, ok := m.GetJobIDByURL("http://127.0.0.1:1/old.bin"); ok {
		t.Fatal("old url still indexed")
	}

	waitForStatus(t, job, StatusCompleted)
	if !fresh.sawRange("bytes=16384-") {
		t.Fatalf("expected a ranged resume from byte 16384, saw %v (status %s err %v)", fresh.ranges, job.GetStatus(), job.GetError())
	}
	got, err := os.ReadFile(job.GetOutputPath())
	if err != nil || !bytes.Equal(got, payload) {
		t.Fatalf("resumed file differs from payload (err %v, %d bytes)", err, len(got))
	}
	_ = path
}

func TestRefreshJobURLSizeMismatchKeepsPartialAndFails(t *testing.T) {
	payload := makePayload(64 * 1024)
	other := newRefreshServer(t, makePayload(64*1024+10), `"v1"`)

	m, outDir := newManagerForTest(t, 1)
	defer m.Shutdown()
	old := "http://127.0.0.1:1/old.bin"
	job, path := expiredJobWithPartial(t, m, outDir, old, payload, 16*1024)

	_, err := m.RefreshJobURL(context.Background(), job.ID, other.URL+"/new.bin", false)
	if !errors.Is(err, ErrRemoteChanged) {
		t.Fatalf("err = %v, want ErrRemoteChanged", err)
	}
	if code, status := Classify(err); code != "remote_changed" || status != http.StatusConflict {
		t.Fatalf("classify = %q/%d", code, status)
	}
	if job.GetURL() != old || job.GetStatus() != StatusError {
		t.Fatalf("job must be untouched on mismatch: url=%q status=%q", job.GetURL(), job.GetStatus())
	}
	if st, e := os.Stat(path); e != nil || st.Size() != 16*1024 {
		t.Fatalf("partial must be kept on mismatch: %v", e)
	}
}

func TestRefreshJobURLETagMismatchFromSidecar(t *testing.T) {
	payload := makePayload(64 * 1024)
	fresh := newRefreshServer(t, payload, `"v2"`)

	m, outDir := newManagerForTest(t, 1)
	defer m.Shutdown()
	old := "http://127.0.0.1:1/old.bin"
	job, path := expiredJobWithPartial(t, m, outDir, old, payload, 16*1024)
	savePartsMeta(path, &partsMeta{
		TotalSize: int64(len(payload)), URL: old, ETag: `"v1"`, Version: partsMetaVersion,
		Segments: []segment{{Index: 0, Start: 0, End: int64(len(payload)) - 1, Done: 16 * 1024}},
	})

	_, err := m.RefreshJobURL(context.Background(), job.ID, fresh.URL+"/new.bin", false)
	if !errors.Is(err, ErrRemoteChanged) {
		t.Fatalf("err = %v, want ErrRemoteChanged (same size, different ETag)", err)
	}
	if _, e := os.Stat(path); e != nil {
		t.Fatalf("partial removed on mismatch: %v", e)
	}
	if _, e := os.Stat(partsFilePath(path)); e != nil {
		t.Fatalf("sidecar removed on mismatch: %v", e)
	}
}

func TestRewriteSidecarURLKeepsProgress(t *testing.T) {
	path := filepath.Join(t.TempDir(), "f.bin")
	old, fresh := "http://old/x", "http://new/x"
	savePartsMeta(path, &partsMeta{
		TotalSize: 100, URL: old, ETag: `"v1"`, Version: partsMetaVersion,
		Segments: []segment{{Index: 0, Start: 0, End: 99, Done: 40}},
	})
	if _, ok := loadPartsMeta(path, 100, fresh); ok {
		t.Fatal("precondition: sidecar is keyed to the old URL")
	}
	if !rewriteSidecarURL(path, fresh) {
		t.Fatal("rewriteSidecarURL reported no sidecar")
	}
	meta, ok := loadPartsMeta(path, 100, fresh)
	if !ok {
		t.Fatal("sidecar must be re-keyed to the new URL so resume finds it")
	}
	if meta.ETag != `"v1"` || meta.Segments[0].Done != 40 {
		t.Fatalf("sidecar progress/validators lost: %+v", meta)
	}
	if rewriteSidecarURL(filepath.Join(t.TempDir(), "none"), fresh) {
		t.Fatal("missing sidecar must report false")
	}
}

func TestRefreshJobURLRestartDiscardsPartial(t *testing.T) {
	payload := makePayload(64 * 1024)
	other := makePayload(48 * 1024)
	fresh := newRefreshServer(t, other, `"v9"`)

	m, outDir := newManagerForTest(t, 1)
	defer m.Shutdown()
	job, _ := expiredJobWithPartial(t, m, outDir, "http://127.0.0.1:1/old.bin", payload, 16*1024)

	res, err := m.RefreshJobURL(context.Background(), job.ID, fresh.URL+"/new.bin", true)
	if err != nil {
		t.Fatalf("restart refresh: %v", err)
	}
	if !res.Restarted || res.Resumed {
		t.Fatalf("result = %+v, want restarted", res)
	}
	waitForStatus(t, job, StatusCompleted)
	got, _ := os.ReadFile(job.GetOutputPath())
	if !bytes.Equal(got, other) {
		t.Fatalf("restarted file must equal the new resource (%d bytes)", len(got))
	}
	if fresh.sawRange("bytes=16384-") {
		t.Fatal("restart must not resume from the old offset")
	}
}

func TestRefreshJobURLRejections(t *testing.T) {
	payload := makePayload(1024)
	good := newRefreshServer(t, payload, "")
	gone := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "gone", http.StatusGone)
	}))
	defer gone.Close()

	m, outDir := newManagerForTest(t, 1)
	defer m.Shutdown()
	job, _ := expiredJobWithPartial(t, m, outDir, "http://127.0.0.1:1/old.bin", payload, 512)

	if _, err := m.RefreshJobURL(context.Background(), "nope", good.URL+"/x", false); !errors.Is(err, ErrNotFound) {
		t.Errorf("unknown job: %v", err)
	}
	if _, err := m.RefreshJobURL(context.Background(), job.ID, "ftp://x/y", false); err == nil {
		t.Error("non-http url must be rejected")
	}
	if _, err := m.RefreshJobURL(context.Background(), job.ID, gone.URL+"/x", false); !errors.Is(err, ErrInvalidURL) {
		t.Errorf("unreachable new link must be an invalid-url error, got %v", err)
	}
	job.SetStatus(StatusRunning)
	if _, err := m.RefreshJobURL(context.Background(), job.ID, good.URL+"/x", false); !errors.Is(err, ErrConflict) {
		t.Errorf("running job must conflict, got %v", err)
	}
}
