package download

import (
	"context"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strconv"
	"sync/atomic"
	"testing"
	"time"
)

func testSlowCfg() slowConfig {
	return slowConfig{
		Window: 100 * time.Millisecond, Sustain: 100 * time.Millisecond,
		Tick: 20 * time.Millisecond, Ratio: 0.2, FloorBps: 1000, MaxRestarts: 5,
	}
}

func TestIsSlowSegment(t *testing.T) {
	cfg := testSlowCfg()
	cases := []struct {
		name   string
		speeds []float64
		idx    int
		want   bool
	}{
		{"far below median", []float64{100, 1e6, 1e6, 1e6}, 0, true},
		{"healthy", []float64{9e5, 1e6, 1e6, 1e6}, 0, false},
		{"zero bytes is a stall not slow", []float64{0, 1e6, 1e6}, 0, false},
		{"unmeasurable siblings ignored", []float64{100, -1, -1}, 0, false},
		{"median below floor", []float64{1, 500, 500}, 0, false},
	}
	for _, c := range cases {
		if got := isSlowSegment(c.speeds, c.idx, cfg); got != c.want {
			t.Errorf("%s: got %v want %v", c.name, got, c.want)
		}
	}
}

// fakeSegments drives a monitor with a manual clock: seg 0 crawls, others fly.
func fakeSegments(cfg slowConfig, limited func() bool) (*slowMonitor, *int, func()) {
	done := make([]atomic.Int64, 4)
	m := newSlowMonitor(cfg, done, limited)
	now := time.Unix(1000, 0)
	m.now = func() time.Time { return now }
	restarts := 0
	var attach func(i int)
	attach = func(i int) {
		m.attach(i, func(error) { restarts++ })
	}
	for i := range done {
		attach(i)
	}
	step := func() {
		now = now.Add(cfg.Tick)
		done[0].Add(1)
		for i := 1; i < 4; i++ {
			done[i].Add(50_000)
		}
		m.evaluate()
		// Simulate the worker reconnecting after a cancel.
		for i, s := range m.segs {
			if !s.active {
				attach(i)
			}
		}
	}
	return m, &restarts, step
}

func TestSlowMonitorRestartsAndCaps(t *testing.T) {
	cfg := testSlowCfg()
	cfg.MaxRestarts = 2
	m, restarts, step := fakeSegments(cfg, nil)
	for i := 0; i < 400; i++ {
		step()
	}
	if *restarts != 2 {
		t.Fatalf("restarts=%d want cap 2 (seg0 restarts=%d)", *restarts, m.segs[0].restarts)
	}
	if m.segs[1].restarts != 0 {
		t.Fatal("healthy segment must not restart")
	}
}

func TestSlowMonitorSkippedWhenLimited(t *testing.T) {
	_, restarts, step := fakeSegments(testSlowCfg(), func() bool { return true })
	for i := 0; i < 100; i++ {
		step()
	}
	if *restarts != 0 {
		t.Fatalf("speed-limited job restarted %d times", *restarts)
	}
}

// pacedServer serves ranges in paced chunks. The very first request starting at
// offset 0 is throttled hard (a per-connection throttle); reconnects are fast.
func pacedServer(payload []byte, throttleFirst bool) (*httptest.Server, *atomic.Int64) {
	var reqs atomic.Int64
	var throttled atomic.Bool
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		reqs.Add(1)
		w.Header().Set("Accept-Ranges", "bytes")
		w.Header().Set("ETag", `"v1"`)
		start, end, ok := parseTestRange(r.Header.Get("Range"), len(payload))
		if !ok {
			w.Header().Set("Content-Length", strconv.Itoa(len(payload)))
			w.Write(payload)
			return
		}
		w.Header().Set("Content-Range", "bytes "+strconv.Itoa(start)+"-"+strconv.Itoa(end)+"/"+strconv.Itoa(len(payload)))
		w.Header().Set("Content-Length", strconv.Itoa(end-start+1))
		w.WriteHeader(http.StatusPartialContent)
		chunk := 16 * 1024
		if throttleFirst && start == 0 && end == len(payload)/4-1 && throttled.CompareAndSwap(false, true) {
			chunk = 256
		}
		body := payload[start : end+1]
		for len(body) > 0 {
			n := min(chunk, len(body))
			if _, err := w.Write(body[:n]); err != nil {
				return
			}
			w.(http.Flusher).Flush()
			body = body[n:]
			time.Sleep(10 * time.Millisecond)
		}
	}))
	return srv, &reqs
}

func runPaced(t *testing.T, throttle bool, mutate func(*Options)) (reqs int64, ok bool) {
	t.Helper()
	payload := makePayload(minSegmentedSize)
	srv, n := pacedServer(payload, throttle)
	defer srv.Close()
	job, opt, dir := ctrlJobOpt(t, srv, payload, 4)
	cfg := testSlowCfg()
	opt.slowCfg = &cfg
	opt.StallTimeout = -1
	if mutate != nil {
		mutate(&opt)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	if err := DownloadSegmented(ctx, opt, job, nil); err != nil {
		t.Fatalf("download: %v", err)
	}
	assertBytesEqual(t, filepath.Join(dir, "file.bin"), payload)
	return n.Load(), true
}

func TestSlowSegmentRestartedAndFileCorrect(t *testing.T) {
	base, _ := runPaced(t, false, nil) // probe + one request per segment
	reqs, _ := runPaced(t, true, nil)
	if reqs <= base {
		t.Fatalf("expected a reconnect for the slow segment, requests=%d base=%d", reqs, base)
	}
}

func TestSlowSegmentNotRestartedWhenSpeedLimited(t *testing.T) {
	// The throttled segment would take ~40s to finish, so observe 2s of the run:
	// without the limit it is reopened within ~0.3s; with it, never.
	payload := makePayload(minSegmentedSize)
	srv, n := pacedServer(payload, true)
	defer srv.Close()
	job, opt, _ := ctrlJobOpt(t, srv, payload, 4)
	cfg := testSlowCfg()
	opt.slowCfg = &cfg
	opt.StallTimeout = -1
	opt.SpeedLimit = 100000
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	_ = DownloadSegmented(ctx, opt, job, nil)
	if got := n.Load(); got != 5 { // probe + one request per segment
		t.Fatalf("speed-limited download reconnected: requests=%d", got)
	}
}
