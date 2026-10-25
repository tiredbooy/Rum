package download

import (
	"context"
	"testing"
)

func TestConnectivityStatus(t *testing.T) {
	m := newQuietManager(t)
	c := NewResumeController(m)
	reachable := map[string]bool{}
	c.probe = func(_ context.Context, target string) bool { return reachable[target] }

	// Nothing to probe yet: optimistic online, no probe timestamp.
	c.refreshConnectivity(context.Background())
	if online, at := c.Status(); !online || !at.IsZero() {
		t.Fatalf("idle status = %v, %v; want online with zero time", online, at)
	}

	addFailedJob(m, "a", "https://a.example/f", nil)
	c.refreshConnectivity(context.Background())
	online, at := c.Status()
	if online || at.IsZero() {
		t.Fatalf("unreachable host: online=%v at=%v; want offline with probe time", online, at)
	}

	// Any reachable host proves the internet works (one dead host is not an outage).
	addFailedJob(m, "b", "https://b.example/f", nil)
	reachable["https://b.example/f"] = true
	c.refreshConnectivity(context.Background())
	if online, _ := c.Status(); !online {
		t.Fatal("one reachable host should mean online")
	}

	// Connectivity lost again.
	reachable["https://b.example/f"] = false
	c.refreshConnectivity(context.Background())
	if online, _ := c.Status(); online {
		t.Fatal("all hosts unreachable should mean offline")
	}

	// A nil controller reports online (no data = no banner).
	var nilC *ResumeController
	if online, _ := nilC.Status(); !online {
		t.Fatal("nil controller must report online")
	}
}

// Offline must not stick once nothing is left to probe (jobs removed or done):
// a stale "offline" would pin the banner forever.
func TestConnectivityStatusClearsWhenNoJobsToProbe(t *testing.T) {
	m := newQuietManager(t)
	c := NewResumeController(m)
	c.probe = func(context.Context, string) bool { return false }

	addFailedJob(m, "a", "https://a.example/f", nil)
	c.refreshConnectivity(context.Background())
	if online, _ := c.Status(); online {
		t.Fatal("setup: expected offline")
	}

	if err := m.DeleteJob("a"); err != nil {
		t.Fatal(err)
	}
	c.refreshConnectivity(context.Background())
	if online, at := c.Status(); !online || !at.IsZero() {
		t.Fatalf("no evidence left: online=%v at=%v; want online, zero time", online, at)
	}
}
