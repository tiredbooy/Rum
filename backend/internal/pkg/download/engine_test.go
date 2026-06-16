package download

import (
	"context"
	"errors"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// ---------------------------------------------------------------------------
// Speed-limiter math
// ---------------------------------------------------------------------------

func TestSpeedLimitBytesPerSec(t *testing.T) {
	cases := []struct {
		kb   int
		want int64
	}{
		{0, 0},
		{-5, 0},
		{1, 1024},
		{100, 102400},
		{1024, 1024 * 1024},
	}
	for _, c := range cases {
		if got := speedLimitBytesPerSec(c.kb); got != c.want {
			t.Errorf("speedLimitBytesPerSec(%d) = %d, want %d", c.kb, got, c.want)
		}
	}
}

func TestNewSpeedLimiter(t *testing.T) {
	if l := newSpeedLimiter(0); l != nil {
		t.Errorf("expected nil limiter for 0 KB/s (unlimited)")
	}
	if l := newSpeedLimiter(-10); l != nil {
		t.Errorf("expected nil limiter for negative KB/s")
	}
	// 100 KB/s => 102400 bytes/s rate.
	l := newSpeedLimiter(100)
	if l == nil {
		t.Fatal("expected a limiter for 100 KB/s")
	}
	if int64(l.Limit()) != 102400 {
		t.Errorf("limiter rate = %v, want 102400 bytes/s", l.Limit())
	}
	// Burst must be at least one read buffer so WaitN(buffer) never fails.
	if l.Burst() < downloadBufferSize {
		t.Errorf("burst %d < downloadBufferSize %d", l.Burst(), downloadBufferSize)
	}
	// A WaitN for a full buffer should be admissible (not exceed burst).
	if err := l.WaitN(context.Background(), downloadBufferSize); err != nil {
		t.Errorf("WaitN(buffer) failed: %v", err)
	}
}

// ---------------------------------------------------------------------------
// Retry / backoff decision: transient vs permanent
// ---------------------------------------------------------------------------

func TestIsRetryableError(t *testing.T) {
	cases := []struct {
		name string
		err  error
		want bool
	}{
		{"nil", nil, false},
		{"context canceled", context.Canceled, false},
		{"deadline exceeded", context.DeadlineExceeded, false},
		{"http 404", &httpStatusError{code: 404}, false},
		{"http 403", &httpStatusError{code: 403}, false},
		{"http 400", &httpStatusError{code: 400}, false},
		{"http 429", &httpStatusError{code: 429}, true},
		{"http 408", &httpStatusError{code: 408}, true},
		{"http 500", &httpStatusError{code: 500}, true},
		{"http 503", &httpStatusError{code: 503}, true},
		{"unexpected eof", io.ErrUnexpectedEOF, true},
		{"connection reset string", errors.New("read tcp: connection reset by peer"), true},
		{"broken pipe string", errors.New("write: broken pipe"), true},
		{"random permanent", errors.New("malformed url"), false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := isRetryableError(c.err); got != c.want {
				t.Errorf("isRetryableError(%v) = %v, want %v", c.err, got, c.want)
			}
		})
	}
}

func TestIsRetryableNetError(t *testing.T) {
	// A net.Error timeout should be retryable.
	var ne net.Error = &net.OpError{Op: "dial", Err: &timeoutErr{}}
	if !isRetryableError(ne) {
		t.Errorf("expected net timeout error to be retryable")
	}
}

type timeoutErr struct{}

func (e *timeoutErr) Error() string   { return "i/o timeout" }
func (e *timeoutErr) Timeout() bool   { return true }
func (e *timeoutErr) Temporary() bool { return true }

func TestBackoffDelayGrowsAndCaps(t *testing.T) {
	base := 100 * time.Millisecond
	maxDelay := 1 * time.Second
	// With full jitter the value is in [0, computed]; verify the cap holds even
	// at high attempt counts.
	for attempt := 0; attempt < 20; attempt++ {
		d := backoffDelay(attempt, base, maxDelay, nil)
		if d < 0 || d > maxDelay {
			t.Errorf("attempt %d: delay %v out of [0,%v]", attempt, d, maxDelay)
		}
	}
}

func TestRetryWithBackoffStopsOnPermanent(t *testing.T) {
	cfg := retryConfig{maxRetries: 5, base: time.Millisecond, maxDelay: time.Millisecond}
	calls := 0
	err := retryWithBackoff(context.Background(), cfg, func(ctx context.Context) error {
		calls++
		return &httpStatusError{code: 404}
	})
	if calls != 1 {
		t.Errorf("permanent error should not retry; got %d calls", calls)
	}
	if err == nil {
		t.Error("expected error")
	}
}

func TestRetryWithBackoffRetriesTransient(t *testing.T) {
	cfg := retryConfig{maxRetries: 3, base: time.Millisecond, maxDelay: time.Millisecond}
	calls := 0
	err := retryWithBackoff(context.Background(), cfg, func(ctx context.Context) error {
		calls++
		if calls < 3 {
			return io.ErrUnexpectedEOF
		}
		return nil
	})
	if err != nil {
		t.Errorf("expected eventual success, got %v", err)
	}
	if calls != 3 {
		t.Errorf("expected 3 calls, got %d", calls)
	}
}

func TestRetryWithBackoffRespectsContext(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cfg := retryConfig{maxRetries: 10, base: time.Hour, maxDelay: time.Hour}
	cancel() // already cancelled
	err := retryWithBackoff(ctx, cfg, func(ctx context.Context) error {
		return io.ErrUnexpectedEOF
	})
	if err == nil {
		t.Error("expected error after cancellation")
	}
}

// ---------------------------------------------------------------------------
// Progress guard
// ---------------------------------------------------------------------------

func TestProgressPercent(t *testing.T) {
	cases := []struct {
		downloaded, total int64
		want              int
	}{
		{0, -1, -1},   // unknown total
		{50, 0, -1},   // zero total -> indeterminate
		{0, 100, 0},   // start
		{50, 100, 50}, // half
		{100, 100, 100},
		{150, 100, 100}, // clamp over 100
		{-5, 100, 0},    // clamp negative
	}
	for _, c := range cases {
		if got := progressPercent(c.downloaded, c.total); got != c.want {
			t.Errorf("progressPercent(%d,%d) = %d, want %d", c.downloaded, c.total, got, c.want)
		}
	}
}

// ---------------------------------------------------------------------------
// Segment range computation
// ---------------------------------------------------------------------------

func TestComputeSegmentsCoversWholeFile(t *testing.T) {
	cases := []struct {
		total int64
		n     int
	}{
		{1000, 4},
		{1001, 4}, // remainder
		{10, 3},
		{5, 5},
		{100, 1},
	}
	for _, c := range cases {
		segs := computeSegments(c.total, c.n)
		if len(segs) == 0 {
			t.Fatalf("total=%d n=%d: no segments", c.total, c.n)
		}
		// First segment starts at 0, last ends at total-1, contiguous, no gaps.
		if segs[0].Start != 0 {
			t.Errorf("total=%d n=%d: first start %d != 0", c.total, c.n, segs[0].Start)
		}
		if segs[len(segs)-1].End != c.total-1 {
			t.Errorf("total=%d n=%d: last end %d != %d", c.total, c.n, segs[len(segs)-1].End, c.total-1)
		}
		var sum int64
		for i, s := range segs {
			if s.End < s.Start {
				t.Errorf("total=%d n=%d: seg %d has end<start", c.total, c.n, i)
			}
			if i > 0 && s.Start != segs[i-1].End+1 {
				t.Errorf("total=%d n=%d: gap/overlap at seg %d", c.total, c.n, i)
			}
			sum += s.size()
		}
		if sum != c.total {
			t.Errorf("total=%d n=%d: segment bytes sum %d != total", c.total, c.n, sum)
		}
	}
}

func TestResolveConnections(t *testing.T) {
	cases := []struct {
		requested int
		total     int64
		want      int
	}{
		{0, 100 << 20, 1},  // 0 => single stream
		{1, 100 << 20, 1},  // 1 => single stream
		{4, 100 << 20, 4},  // plenty of size
		{8, 100 << 20, 8},  // plenty of size
		{100, 100 << 20, maxConnections}, // capped at maxConnections
		{4, 0, 1},          // unknown size => single
		{8, 2 << 20, 2},    // 2 MiB with 1 MiB min segment => 2 segments
	}
	for _, c := range cases {
		if got := resolveConnections(c.requested, c.total); got != c.want {
			t.Errorf("resolveConnections(%d,%d) = %d, want %d", c.requested, c.total, got, c.want)
		}
	}
}

func TestShouldSegment(t *testing.T) {
	big := int64(minSegmentedSize * 4)
	if shouldSegment(false, big, 4) {
		t.Error("should not segment without range support")
	}
	if shouldSegment(true, minSegmentedSize-1, 4) {
		t.Error("should not segment below threshold")
	}
	if shouldSegment(true, big, 1) {
		t.Error("should not segment with 1 connection")
	}
	if !shouldSegment(true, big, 4) {
		t.Error("should segment for a large range-capable file with >1 connections")
	}
}

// ---------------------------------------------------------------------------
// End-to-end segmented download via httptest
// ---------------------------------------------------------------------------

// rangeServer serves payload, honoring Range requests and Accept-Ranges.
func rangeServer(t *testing.T, payload []byte) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Accept-Ranges", "bytes")
		http.ServeContent(w, r, "file.bin", time.Now(), newReadSeeker(payload))
	}))
}

type readSeeker struct {
	data []byte
	off  int64
}

func newReadSeeker(b []byte) *readSeeker { return &readSeeker{data: b} }
func (r *readSeeker) Read(p []byte) (int, error) {
	if r.off >= int64(len(r.data)) {
		return 0, io.EOF
	}
	n := copy(p, r.data[r.off:])
	r.off += int64(n)
	return n, nil
}
func (r *readSeeker) Seek(offset int64, whence int) (int64, error) {
	switch whence {
	case io.SeekStart:
		r.off = offset
	case io.SeekCurrent:
		r.off += offset
	case io.SeekEnd:
		r.off = int64(len(r.data)) + offset
	}
	return r.off, nil
}

func makePayload(n int) []byte {
	b := make([]byte, n)
	for i := range b {
		b[i] = byte(i % 251)
	}
	return b
}

func newTestJobAndOpt(t *testing.T, srv *httptest.Server, payload []byte, connections int) (*Job, Options, string) {
	t.Helper()
	dir := t.TempDir()
	job := &Job{
		ID:           "test",
		URL:          srv.URL + "/file.bin",
		FileName:     "file.bin",
		TotalSize:    int64(len(payload)),
		SupportRange: true,
		Status:       StatusPending,
	}
	opt := Options{
		Out:         dir,
		Connections: connections,
		Downloader:  NewDownloader("test-agent", ""),
		MaxRetries:  2,
	}
	return job, opt, dir
}

func TestDownloadSegmentedEndToEnd(t *testing.T) {
	payload := makePayload(minSegmentedSize * 3) // big enough to segment
	srv := rangeServer(t, payload)
	defer srv.Close()

	job, opt, dir := newTestJobAndOpt(t, srv, payload, 6)

	var lastDownloaded, lastTotal int64
	progress := func(d, total int64) { lastDownloaded, lastTotal = d, total }

	if err := DownloadSegmented(context.Background(), opt, job, progress); err != nil {
		t.Fatalf("DownloadSegmented failed: %v", err)
	}

	out := filepath.Join(dir, "file.bin")
	got, err := os.ReadFile(out)
	if err != nil {
		t.Fatalf("read output: %v", err)
	}
	if len(got) != len(payload) {
		t.Fatalf("size mismatch: got %d want %d", len(got), len(payload))
	}
	for i := range got {
		if got[i] != payload[i] {
			t.Fatalf("byte mismatch at %d", i)
		}
	}
	if lastTotal != int64(len(payload)) || lastDownloaded != int64(len(payload)) {
		t.Errorf("final progress = %d/%d, want %d/%d", lastDownloaded, lastTotal, len(payload), len(payload))
	}
	// Sidecar must be cleaned up on success.
	if _, err := os.Stat(partsFilePath(out)); !os.IsNotExist(err) {
		t.Errorf("expected parts sidecar removed on success")
	}
}

func TestDownloadSegmentedResume(t *testing.T) {
	payload := makePayload(minSegmentedSize * 2)
	srv := rangeServer(t, payload)
	defer srv.Close()

	job, opt, dir := newTestJobAndOpt(t, srv, payload, 4)
	out := filepath.Join(dir, "file.bin")

	// Pre-create a partially written output + a parts sidecar that marks the
	// first half of each segment as done with the CORRECT bytes already on
	// disk, leaving the rest to be fetched.
	segs := computeSegments(int64(len(payload)), 4)
	if err := os.WriteFile(out, make([]byte, len(payload)), 0644); err != nil {
		t.Fatal(err)
	}
	f, err := os.OpenFile(out, os.O_WRONLY, 0644)
	if err != nil {
		t.Fatal(err)
	}
	for i := range segs {
		half := segs[i].size() / 2
		segs[i].Done = half
		// Write the real bytes for the "already done" prefix.
		if _, err := f.WriteAt(payload[segs[i].Start:segs[i].Start+half], segs[i].Start); err != nil {
			t.Fatal(err)
		}
	}
	f.Close()
	savePartsMeta(out, &partsMeta{TotalSize: int64(len(payload)), URL: job.URL, Segments: segs})

	if err := DownloadSegmented(context.Background(), opt, job, nil); err != nil {
		t.Fatalf("resume failed: %v", err)
	}
	got, err := os.ReadFile(out)
	if err != nil {
		t.Fatal(err)
	}
	for i := range got {
		if got[i] != payload[i] {
			t.Fatalf("resume byte mismatch at %d", i)
		}
	}
}

func TestDownloadSegmentedFallsBackSmallFile(t *testing.T) {
	payload := makePayload(1024) // below threshold
	srv := rangeServer(t, payload)
	defer srv.Close()

	job, opt, dir := newTestJobAndOpt(t, srv, payload, 8)
	if err := DownloadSegmented(context.Background(), opt, job, nil); err != nil {
		t.Fatalf("fallback download failed: %v", err)
	}
	got, err := os.ReadFile(filepath.Join(dir, "file.bin"))
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != len(payload) {
		t.Fatalf("size mismatch: got %d want %d", len(got), len(payload))
	}
	for i := range got {
		if got[i] != payload[i] {
			t.Fatalf("byte mismatch at %d", i)
		}
	}
}
