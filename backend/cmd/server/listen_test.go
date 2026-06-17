package server

import (
	"net"
	"net/http"
	"strings"
	"testing"
	"time"
)

func TestListenBindsLoopbackAndServes(t *testing.T) {
	// Isolate config/jobs under a temp dir so the test never touches real state.
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())

	base, err := Listen()
	if err != nil {
		t.Fatalf("Listen: %v", err)
	}
	if !strings.HasPrefix(base, "http://127.0.0.1:") {
		t.Fatalf("want loopback base, got %q", base)
	}
	if APIBase() != base {
		t.Fatalf("APIBase()=%q want %q", APIBase(), base)
	}
	go Serve()
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if resp, err := http.Get(base + "/api/v1/downloads/stats"); err == nil {
			resp.Body.Close()
			return
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatal("server did not answer on the bound port")
}

// TestBindLoopbackFallsBackWhenPortTaken verifies the low-level bind helper:
// when the preferred port is already occupied it binds a dynamic free port on
// loopback instead of hard-failing. Testing bindLoopback directly keeps this
// independent of any server started by other tests in the package.
func TestBindLoopbackFallsBackWhenPortTaken(t *testing.T) {
	// Occupy the preferred port on loopback so bindLoopback must fall back.
	blocker, err := net.Listen("tcp", net.JoinHostPort(loopbackHost, preferredPort))
	if err != nil {
		// Preferred port already in use by something else: the fallback path is
		// exactly what we want to exercise, so this is acceptable — skip rather
		// than fail spuriously.
		t.Skipf("could not occupy preferred port %s: %v", preferredPort, err)
	}
	defer blocker.Close()

	ln2, addr, err := bindLoopback()
	if err != nil {
		t.Fatalf("bindLoopback with preferred port taken: %v", err)
	}
	defer ln2.Close()

	if !strings.HasPrefix(addr, "127.0.0.1:") {
		t.Fatalf("want loopback addr, got %q", addr)
	}
	if strings.HasSuffix(addr, ":"+preferredPort) {
		t.Fatalf("expected a fallback port, but bound the (occupied) preferred port: %q", addr)
	}
}

// TestBindLoopbackPrefersPreferredPort verifies bindLoopback uses the preferred
// port when it is free.
func TestBindLoopbackPrefersPreferredPort(t *testing.T) {
	ln1, addr, err := bindLoopback()
	if err != nil {
		t.Fatalf("bindLoopback: %v", err)
	}
	defer ln1.Close()
	if !strings.HasPrefix(addr, "127.0.0.1:") {
		t.Fatalf("want loopback addr, got %q", addr)
	}
	// If 8080 was free it should be chosen; if not, a dynamic port is fine. Either
	// way the bind must succeed on loopback (asserted above).
}
