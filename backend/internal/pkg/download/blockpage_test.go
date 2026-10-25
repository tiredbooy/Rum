package download

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
)

const portalHTML = "<!DOCTYPE html><html><body>Please log in to the network</body></html>"

type blockFixture struct {
	job  *Job
	opt  Options
	path string
	hits *atomic.Int32
}

// newBlockFixture serves handler for name under a temp dir with an optional
// pre-existing partial of the given bytes.
func newBlockFixture(t *testing.T, name string, total int64, partial []byte, h http.HandlerFunc) blockFixture {
	t.Helper()
	hits := &atomic.Int32{}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits.Add(1)
		h(w, r)
	}))
	t.Cleanup(srv.Close)
	dir := t.TempDir()
	path := filepath.Join(dir, name)
	if partial != nil {
		if err := os.WriteFile(path, partial, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return blockFixture{
		job: &Job{ID: "bp", URL: srv.URL + "/" + name, FileName: name, TotalSize: total,
			SupportRange: true, ContentType: "application/zip", Status: StatusPending},
		opt:  Options{Out: dir, Connections: 1, Downloader: NewDownloader("t", ""), MaxRetries: 3},
		path: path,
		hits: hits,
	}
}

func htmlHandler(ct string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if ct == "" {
			w.Header()["Content-Type"] = nil
		} else {
			w.Header().Set("Content-Type", ct)
		}
		_, _ = w.Write([]byte(portalHTML))
	}
}

func TestBlockedPageOnResumeLeavesPartialUntouched(t *testing.T) {
	partial := bytes.Repeat([]byte{7}, 1000)
	f := newBlockFixture(t, "file.zip", 5000, partial, htmlHandler("text/html; charset=utf-8"))

	err := DownloadSingleFile(context.Background(), f.opt, f.job, nil)
	var bp *blockedPageError
	if !errors.As(err, &bp) {
		t.Fatalf("want blockedPageError, got %v", err)
	}
	if got, _ := os.ReadFile(f.path); !bytes.Equal(got, partial) {
		t.Fatalf("partial was modified: %d bytes", len(got))
	}
	if n := f.hits.Load(); n != 1 {
		t.Fatalf("blocked page must not be retried, got %d requests", n)
	}
	if isRetryableError(err) {
		t.Fatal("blocked page must not be retryable")
	}
	if !strings.Contains(err.Error(), "web page") {
		t.Fatalf("unexpected message %q", err)
	}
}

func TestBlockedPageSniffedWhenTypeGeneric(t *testing.T) {
	for _, ct := range []string{"application/octet-stream", ""} {
		f := newBlockFixture(t, "file.zip", 5000, nil, htmlHandler(ct))
		err := DownloadSingleFile(context.Background(), f.opt, f.job, nil)
		var bp *blockedPageError
		if !errors.As(err, &bp) {
			t.Fatalf("ct=%q: want blockedPageError, got %v", ct, err)
		}
		if fi, err := os.Stat(f.path); err == nil && fi.Size() != 0 {
			t.Fatalf("ct=%q: wrote %d bytes", ct, fi.Size())
		}
	}
}

func TestBinaryBodyWithGenericTypeIsNotBlocked(t *testing.T) {
	payload := makePayload(4096)
	f := newBlockFixture(t, "file.zip", int64(len(payload)), nil, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/octet-stream")
		_, _ = w.Write(payload)
	})
	if err := DownloadSingleFile(context.Background(), f.opt, f.job, nil); err != nil {
		t.Fatal(err)
	}
}

func TestRealHTMLDownloadStillWorks(t *testing.T) {
	f := newBlockFixture(t, "page.html", int64(len(portalHTML)), nil, htmlHandler("text/html"))
	f.job.ContentType = "text/html"
	if err := DownloadSingleFile(context.Background(), f.opt, f.job, nil); err != nil {
		t.Fatal(err)
	}
	if got, _ := os.ReadFile(f.path); string(got) != portalHTML {
		t.Fatalf("content mismatch: %q", got)
	}
}

func TestBlockedPageOverrideWritesHTML(t *testing.T) {
	f := newBlockFixture(t, "file.zip", int64(len(portalHTML)), nil, htmlHandler("text/html"))
	f.job.SetAllowBlockedPage(true)
	if err := DownloadSingleFile(context.Background(), f.opt, f.job, nil); err != nil {
		t.Fatal(err)
	}
	if got, _ := os.ReadFile(f.path); string(got) != portalHTML {
		t.Fatalf("override did not write body: %q", got)
	}
}

func TestBlockedPageSegmentedRangeGets200HTML(t *testing.T) {
	total := int64(minSegmentedSize) * 2
	f := newBlockFixture(t, "big.zip", total, nil, htmlHandler("text/html"))
	f.opt.Connections = 4
	err := DownloadSegmented(context.Background(), f.opt, f.job, nil)
	var bp *blockedPageError
	if !errors.As(err, &bp) {
		t.Fatalf("want blockedPageError, got %v", err)
	}
}

func TestSetErrorRecordsBlockedPageCode(t *testing.T) {
	j := &Job{ID: "x"}
	j.SetError(&blockedPageError{})
	if j.ErrorCode != ErrCodeBlockedPage {
		t.Fatalf("code = %q", j.ErrorCode)
	}
	j.FinalHost = "portal.example"
	b, err := json.Marshal(j)
	if err != nil {
		t.Fatal(err)
	}
	var back Job
	if err := json.Unmarshal(b, &back); err != nil {
		t.Fatal(err)
	}
	if back.ErrorCode != ErrCodeBlockedPage || back.FinalHost != "portal.example" {
		t.Fatalf("roundtrip lost fields: %s", b)
	}
	j.SetError(nil)
	b, _ = json.Marshal(j)
	if strings.Contains(string(b), "error_code") || strings.Contains(string(b), "final_host") && j.FinalHost == "" {
		t.Fatalf("omitempty violated: %s", b)
	}
}

func TestParseHeaderInfoRecordsFinalHost(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	defer srv.Close()
	resp, err := http.Get(srv.URL)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if got := ParseHeaderInfo(resp).FinalHost; got != strings.TrimPrefix(srv.URL, "http://") {
		t.Fatalf("FinalHost = %q", got)
	}
}

func TestReconnectSkipsBlockedPageJobs(t *testing.T) {
	j := &Job{}
	j.SetError(&blockedPageError{})
	if !jobBlocked(j) {
		t.Fatal("job with blocked_page code must be skipped by the reconnect controller")
	}
}

func rangeUnsatisfiable(contentRange string, full []byte) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Range") != "" {
			if contentRange != "" {
				w.Header().Set("Content-Range", contentRange)
			}
			w.WriteHeader(http.StatusRequestedRangeNotSatisfiable)
			return
		}
		w.Header().Set("Content-Type", "application/octet-stream")
		_, _ = w.Write(full)
	}
}

func TestRange416CompleteWhenLocalMatchesKnownTotal(t *testing.T) {
	local := makePayload(2048)
	f := newBlockFixture(t, "file.zip", int64(len(local)), local, rangeUnsatisfiable("", nil))
	if err := downloadSingleAttempt(context.Background(), f.opt, f.job, f.job.URL, f.path, nil); err != nil {
		t.Fatal(err)
	}
	if f.job.GetStatus() != StatusCompleted {
		t.Fatalf("status = %s", f.job.GetStatus())
	}
	if got, _ := os.ReadFile(f.path); !bytes.Equal(got, local) {
		t.Fatal("file changed")
	}
}

func TestRange416CompleteWhenContentRangeTotalMatches(t *testing.T) {
	local := makePayload(2048)
	f := newBlockFixture(t, "file.zip", -1, local, rangeUnsatisfiable("bytes */2048", nil))
	if err := downloadSingleAttempt(context.Background(), f.opt, f.job, f.job.URL, f.path, nil); err != nil {
		t.Fatal(err)
	}
	if f.job.GetStatus() != StatusCompleted {
		t.Fatalf("status = %s", f.job.GetStatus())
	}
}

func TestRange416RestartsWhenTotalUnknownOrMismatch(t *testing.T) {
	full := makePayload(3000)
	for name, tc := range map[string]struct {
		total int64
		cr    string
	}{
		"unknown total":       {-1, ""},
		"size mismatch":       {3000, ""},
		"content-range other": {-1, "bytes */3000"},
	} {
		t.Run(name, func(t *testing.T) {
			stale := bytes.Repeat([]byte{9}, 2048)
			f := newBlockFixture(t, "file.zip", tc.total, stale, rangeUnsatisfiable(tc.cr, full))
			if err := downloadSingleAttempt(context.Background(), f.opt, f.job, f.job.URL, f.path, nil); err != nil {
				t.Fatal(err)
			}
			got, _ := os.ReadFile(f.path)
			if !bytes.Equal(got, full) {
				t.Fatalf("expected clean restart, got %d bytes", len(got))
			}
			if f.job.GetStatus() == StatusCompleted {
				t.Fatal("must not be marked complete on a size mismatch before data is verified")
			}
		})
	}
}

func TestManagerBlockedPageThenDownloadAnyway(t *testing.T) {
	m, outDir := newManagerForTest(t, 1)
	defer m.Shutdown()

	srv := httptest.NewServer(htmlHandler("text/html"))
	defer srv.Close()
	partial := bytes.Repeat([]byte{1}, 100)
	path := filepath.Join(outDir, "file.zip")
	if err := os.WriteFile(path, partial, 0o644); err != nil {
		t.Fatal(err)
	}
	job := &Job{ID: "m1", URL: srv.URL + "/file.zip", FileName: "file.zip", TotalSize: 5000,
		ContentType: "application/zip", SupportRange: true, Status: StatusPending}
	m.mu.Lock()
	m.jobs[job.ID] = job
	m.mu.Unlock()

	done := make(chan struct{}, 4)
	prev := setTestCompletionHook(func(string) { done <- struct{}{} })
	defer setTestCompletionHook(prev)

	if err := m.StartJob(context.Background(), job.ID); err != nil {
		t.Fatal(err)
	}
	<-done
	if job.GetStatus() != StatusError || job.GetErrorCode() != ErrCodeBlockedPage {
		t.Fatalf("status=%s code=%q err=%v", job.GetStatus(), job.GetErrorCode(), job.GetError())
	}
	if got, _ := os.ReadFile(path); !bytes.Equal(got, partial) {
		t.Fatal("partial modified by blocked attempt")
	}

	if err := m.DownloadAnyway(context.Background(), job.ID); err != nil {
		t.Fatal(err)
	}
	<-done
	if job.GetStatus() != StatusCompleted || job.GetErrorCode() != "" {
		t.Fatalf("after override: status=%s code=%q err=%v", job.GetStatus(), job.GetErrorCode(), job.GetError())
	}
	if job.GetAllowBlockedPage() {
		t.Fatal("override must be one-time")
	}
}
