package download

import (
	"context"
	"errors"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/tiredbooy/Rum/backend/internal/pkg/config"
)

// fakeNet answers probes from a per-URL table; unknown URLs are unreachable.
// isolateHome gives the test its own config/data dirs so persisted jobs from
// other tests are not loaded into the manager.
func isolateHome(t *testing.T) {
	t.Helper()
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	t.Setenv("XDG_DATA_HOME", t.TempDir())
}

func fakeNet(up ...string) func(context.Context, string) bool {
	ok := map[string]bool{}
	for _, u := range up {
		ok[u] = true
	}
	return func(_ context.Context, target string) bool { return ok[target] }
}

func errTimeoutForTest() error {
	return &net.OpError{Op: "dial", Err: errors.New("connection refused")}
}

var testEndpoints = []string{"https://g1.example/", "https://g2.example/"}

func TestClassifyConnectivity(t *testing.T) {
	jobs := []string{"https://blocked.example/f", "https://fine.example/f"}
	cases := []struct {
		name        string
		up          []string
		wantClass   ConnClass
		wantBlocked []string
	}{
		{"all reachable", []string{"https://g1.example/", "https://blocked.example/f", "https://fine.example/f"}, ConnOnline, nil},
		{"one site filtered, internet works", []string{"https://g2.example/", "https://fine.example/f"}, ConnSiteBlocked, []string{"https://blocked.example/f"}},
		{"nothing answers", nil, ConnOffline, nil},
		{"a job host alone proves internet", []string{"https://fine.example/f"}, ConnSiteBlocked, []string{"https://blocked.example/f"}},
		{"only general endpoints up: job hosts both blocked", []string{"https://g1.example/"}, ConnSiteBlocked, []string{"https://blocked.example/f", "https://fine.example/f"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			class, blocked := classifyConnectivity(context.Background(), fakeNet(tc.up...), jobs, testEndpoints)
			if class != tc.wantClass {
				t.Fatalf("class = %q, want %q", class, tc.wantClass)
			}
			if len(blocked) != len(tc.wantBlocked) {
				t.Fatalf("blocked = %v, want %v", blocked, tc.wantBlocked)
			}
			for _, w := range tc.wantBlocked {
				if !blocked[w] {
					t.Fatalf("blocked = %v, missing %s", blocked, w)
				}
			}
		})
	}
}

func TestClassifyConnectivityWithoutJobsUsesGeneralEndpoints(t *testing.T) {
	if class, _ := classifyConnectivity(context.Background(), fakeNet("https://g2.example/"), nil, testEndpoints); class != ConnOnline {
		t.Fatalf("class = %q, want online", class)
	}
	if class, _ := classifyConnectivity(context.Background(), fakeNet(), nil, testEndpoints); class != ConnOffline {
		t.Fatalf("class = %q, want offline", class)
	}
}

// All probes run at the same time and the whole check honours the deadline, so
// a pile of slow/blackholed endpoints cannot stall the ticker goroutine.
func TestClassifyConnectivityIsConcurrentAndBounded(t *testing.T) {
	var inflight, peak int32
	var mu sync.Mutex
	slow := func(ctx context.Context, _ string) bool {
		n := atomic.AddInt32(&inflight, 1)
		mu.Lock()
		if n > peak {
			peak = n
		}
		mu.Unlock()
		defer atomic.AddInt32(&inflight, -1)
		select {
		case <-ctx.Done():
		case <-time.After(10 * time.Second):
		}
		return false
	}
	old := connectivityDeadline
	connectivityDeadline = 150 * time.Millisecond
	t.Cleanup(func() { connectivityDeadline = old })

	start := time.Now()
	class, _ := classifyConnectivity(context.Background(), slow,
		[]string{"https://a.example/", "https://b.example/"}, testEndpoints)
	if el := time.Since(start); el > 2*time.Second {
		t.Fatalf("took %v, want about the deadline", el)
	}
	if class != ConnOffline {
		t.Fatalf("class = %q, want offline", class)
	}
	if peak != 4 {
		t.Fatalf("peak concurrency = %d, want all 4 probes in flight together", peak)
	}
}

func TestControllerMarksBlockedSiteAndKeepsBannerOnline(t *testing.T) {
	isolateHome(t)
	m := newQuietManager(t)
	c := NewResumeController(m)
	c.endpoints = func() []string { return testEndpoints }
	c.probe = fakeNet("https://g1.example/")

	job := addFailedJob(m, "a", "https://blocked.example/f", nil)
	job.SetError(errTimeoutForTest())
	c.refreshConnectivity(context.Background())

	if online, at := c.Status(); !online || at.IsZero() {
		t.Fatalf("online=%v at=%v; a blocked site must not read as offline", online, at)
	}
	if c.ConnState() != string(ConnSiteBlocked) {
		t.Fatalf("state = %q", c.ConnState())
	}
	code, retry := job.GetErrorCode()
	if code != ErrCodeSiteUnreachable || !retry {
		t.Fatalf("code=%q retry=%v; want site_unreachable, retryable", code, retry)
	}
	if !jobAutoResumable(job) {
		t.Fatal("a blocked-site job must stay auto-resumable once the site returns")
	}

	// Site recovers: the code is dropped back to a plain network class.
	c.probe = fakeNet("https://g1.example/", "https://blocked.example/f")
	c.refreshConnectivity(context.Background())
	if c.ConnState() != string(ConnOnline) {
		t.Fatalf("state = %q, want online", c.ConnState())
	}
	if code, _ := job.GetErrorCode(); code != ErrCodeNetworkDown {
		t.Fatalf("code = %q after recovery, want the original network_down", code)
	}
}

func TestControllerOfflineLeavesJobCodesAlone(t *testing.T) {
	isolateHome(t)
	m := newQuietManager(t)
	c := NewResumeController(m)
	c.endpoints = func() []string { return testEndpoints }
	c.probe = fakeNet()

	job := addFailedJob(m, "a", "https://x.example/f", nil)
	job.SetError(errTimeoutForTest())
	c.refreshConnectivity(context.Background())
	if online, _ := c.Status(); online {
		t.Fatal("want offline")
	}
	if code, _ := job.GetErrorCode(); code == ErrCodeSiteUnreachable {
		t.Fatal("real offline must not blame the site")
	}
}

// Idle controller: only probes when the UI asked recently or a job is queued.
func TestIdleProbeOnlyWhileDemanded(t *testing.T) {
	isolateHome(t)
	m := newQuietManager(t)
	c := NewResumeController(m)
	c.endpoints = func() []string { return testEndpoints }
	var calls int32
	c.probe = func(context.Context, string) bool { atomic.AddInt32(&calls, 1); return true }

	c.refreshConnectivity(context.Background())
	if atomic.LoadInt32(&calls) != 0 {
		t.Fatal("no jobs and nobody watching: must not probe")
	}

	c.Touch() // the window polled /status
	c.refreshConnectivity(context.Background())
	if atomic.LoadInt32(&calls) == 0 {
		t.Fatal("window visible: want a probe")
	}
	if c.ConnState() != string(ConnOnline) {
		t.Fatalf("state = %q", c.ConnState())
	}

	// Stale demand stops probing again.
	atomic.StoreInt32(&calls, 0)
	c.demandAt.Store(c.now().Add(-2 * demandWindow).UnixNano())
	c.refreshConnectivity(context.Background())
	if atomic.LoadInt32(&calls) != 0 {
		t.Fatal("window hidden: must not probe")
	}

	// A queued (pending) job counts as demand.
	m.mu.Lock()
	m.jobs["q"] = &Job{ID: "q", URL: "https://q.example/f", Status: StatusPending}
	m.mu.Unlock()
	c.refreshConnectivity(context.Background())
	if atomic.LoadInt32(&calls) == 0 {
		t.Fatal("queued download: want a probe")
	}
}

func TestProbeEndpointsConfigOverride(t *testing.T) {
	writeReconnectSettings(t, true)
	if got := probeEndpoints(); len(got) < 4 {
		t.Fatalf("defaults = %v; want a mixed list", got)
	}
	var s config.Setting
	if err := s.LoadSettingMetadata(); err != nil {
		t.Fatal(err)
	}
	s.ProbeEndpoints = []string{" https://mine.example/ ", "not a url", ""}
	if err := s.Save(); err != nil {
		t.Fatal(err)
	}
	got := probeEndpoints()
	if len(got) != 1 || got[0] != "https://mine.example/" {
		t.Fatalf("override = %v", got)
	}
}

// The real probe goes through the configured proxy: the unreachable-by-DNS
// target below can only be answered if the request was sent to the proxy.
func TestDialProbeUsesConfiguredProxy(t *testing.T) {
	var hits int32
	proxy := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&hits, 1)
		w.WriteHeader(http.StatusNoContent)
	}))
	defer proxy.Close()

	writeReconnectSettings(t, true)
	var s config.Setting
	if err := s.LoadSettingMetadata(); err != nil {
		t.Fatal(err)
	}
	u, _ := url.Parse(proxy.URL)
	s.Proxy = u.Host
	if err := s.Save(); err != nil {
		t.Fatal(err)
	}
	if !dialProbe(context.Background(), "http://no-such-host.invalid/generate_204") {
		t.Fatal("probe should succeed via the proxy")
	}
	if atomic.LoadInt32(&hits) != 1 {
		t.Fatalf("proxy hits = %d, want 1", hits)
	}
}
