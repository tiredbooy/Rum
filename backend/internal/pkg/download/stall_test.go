package download

import (
	"bytes"
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"sync/atomic"
	"testing"
	"time"
)

// stallServer serves payload with Range support, but the first `stalls` requests
// send a third of their range and then hang with the connection open.
func stallServer(t *testing.T, payload []byte, stalls int64) (*httptest.Server, *atomic.Int64) {
	t.Helper()
	var reqs atomic.Int64
	release := make(chan struct{})
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		n := reqs.Add(1)
		w.Header().Set("Accept-Ranges", "bytes")
		start, end, status := 0, len(payload)-1, http.StatusOK
		if rh := r.Header.Get("Range"); rh != "" {
			s, e, ok := parseTestRange(rh, len(payload))
			if !ok {
				w.WriteHeader(http.StatusRequestedRangeNotSatisfiable)
				return
			}
			start, end, status = s, e, http.StatusPartialContent
			w.Header().Set("Content-Range", fmt.Sprintf("bytes %d-%d/%d", s, e, len(payload)))
		}
		body := payload[start : end+1]
		w.Header().Set("Content-Length", strconv.Itoa(len(body)))
		w.WriteHeader(status)
		if n <= stalls {
			w.Write(body[:len(body)/3])
			w.(http.Flusher).Flush()
			select {
			case <-r.Context().Done():
			case <-release:
			}
			return
		}
		w.Write(body)
	}))
	t.Cleanup(func() { close(release); srv.Close() })
	return srv, &reqs
}

func stallJobOpt(t *testing.T, srv *httptest.Server, size, conns int) (*Job, Options, string) {
	t.Helper()
	dir := t.TempDir()
	job := &Job{ID: "stall", URL: srv.URL + "/file.bin", FileName: "file.bin",
		TotalSize: int64(size), SupportRange: true, Status: StatusPending}
	opt := Options{Out: dir, Connections: conns, Downloader: NewDownloader("test-agent", ""),
		MaxRetries: 3, RetryBackoffSec: 1, StallTimeout: 300 * time.Millisecond}
	return job, opt, dir
}

func assertFile(t *testing.T, path string, want []byte) {
	t.Helper()
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, want) {
		t.Fatalf("content mismatch: got %d bytes want %d", len(got), len(want))
	}
}

func TestSingleStalledConnectionReconnects(t *testing.T) {
	payload := makePayload(300 * 1024)
	srv, reqs := stallServer(t, payload, 1)
	job, opt, dir := stallJobOpt(t, srv, len(payload), 1)
	before := runtime.NumGoroutine()

	if err := DownloadSingleFile(context.Background(), opt, job, nil); err != nil {
		t.Fatalf("expected recovery after stall, got %v", err)
	}
	assertFile(t, filepath.Join(dir, "file.bin"), payload)
	if reqs.Load() < 2 {
		t.Fatalf("expected a reconnect, requests=%d", reqs.Load())
	}
	time.Sleep(200 * time.Millisecond)
	if after := runtime.NumGoroutine(); after > before+5 {
		t.Fatalf("goroutine leak: before=%d after=%d", before, after)
	}
}

func TestSegmentedStalledConnectionReconnects(t *testing.T) {
	payload := makePayload(2 * 1024 * 1024)
	srv, reqs := stallServer(t, payload, 2)
	job, opt, dir := stallJobOpt(t, srv, len(payload), 4)

	if err := DownloadSegmented(context.Background(), opt, job, nil); err != nil {
		t.Fatalf("expected recovery after stall, got %v", err)
	}
	assertFile(t, filepath.Join(dir, "file.bin"), payload)
	if reqs.Load() < 3 {
		t.Fatalf("expected reconnects, requests=%d", reqs.Load())
	}
}

func TestStallWithoutRetriesReportsStalled(t *testing.T) {
	payload := makePayload(300 * 1024)
	srv, _ := stallServer(t, payload, 5)
	job, opt, _ := stallJobOpt(t, srv, len(payload), 1)
	opt.MaxRetries = 0
	err := DownloadSingleFile(context.Background(), opt, job, nil)
	if err == nil || err.Error() != "stalled, reconnecting" {
		t.Fatalf("expected errStalled, got %v", err)
	}
	if !isRetryableError(err) {
		t.Fatal("errStalled must be retryable")
	}
}

// A steady transfer slowed by the speed limiter spends long stretches in the
// limiter wait (longer than the stall timeout); that must not count as a stall.
func TestRateLimiterWaitIsNotStall(t *testing.T) {
	payload := makePayload(768 * 1024)
	srv, reqs := stallServer(t, payload, 0)
	job, opt, dir := stallJobOpt(t, srv, len(payload), 1)
	opt.SpeedLimit = 256 // KB/s -> ~0.5s limiter wait per 128 KiB chunk
	opt.StallTimeout = 250 * time.Millisecond
	opt.MaxRetries = 0

	if err := DownloadSingleFile(context.Background(), opt, job, nil); err != nil {
		t.Fatalf("rate-limited transfer must not stall: %v", err)
	}
	assertFile(t, filepath.Join(dir, "file.bin"), payload)
	if reqs.Load() != 1 {
		t.Fatalf("expected 1 request, got %d", reqs.Load())
	}
}

func TestStallSetsErrorCodeWhileRetrying(t *testing.T) {
	payload := bytes.Repeat([]byte("abcdefgh"), 64<<10)
	srv, _ := stallServer(t, payload, 1)
	job, opt, dir := stallJobOpt(t, srv, len(payload), 1)
	opt.RetryBackoffSec = 2

	done := make(chan error, 1)
	go func() { done <- DownloadSingleFile(context.Background(), opt, job, nil) }()

	deadline := time.Now().Add(5 * time.Second)
	for job.GetErrorCode() != ErrorCodeStalled && time.Now().Before(deadline) {
		time.Sleep(20 * time.Millisecond)
	}
	if job.GetErrorCode() != ErrorCodeStalled {
		t.Fatal("error code never set to stalled while retrying")
	}
	if err := <-done; err != nil {
		t.Fatal(err)
	}
	if c := job.GetErrorCode(); c != "" {
		t.Fatalf("error code not cleared after recovery: %q", c)
	}
	assertFile(t, filepath.Join(dir, "file.bin"), payload)
}
