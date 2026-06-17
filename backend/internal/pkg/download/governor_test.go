package download

import (
	"bytes"
	"context"
	"io"
	"testing"
	"time"
)

// nopReadCloser wraps a reader with a no-op Close so we can feed Wrap.
type nopReadCloser struct{ io.Reader }

func (nopReadCloser) Close() error { return nil }

func TestGovernorNilReturnsReaderUnchanged(t *testing.T) {
	// The engine/CLI fallback: no governor => no throttling wrapper at all.
	var g *SpeedGovernor
	rc := nopReadCloser{bytes.NewReader([]byte("hello"))}
	if got := g.Wrap(context.Background(), rc); got != rc {
		t.Fatalf("nil governor must return the reader unchanged")
	}
}

func TestGovernorUnlimitedPassesBytesThrough(t *testing.T) {
	// A real governor currently at 0 (unlimited) wraps the reader (so it can pick
	// up a later limit) but must not throttle while unlimited.
	g := NewSpeedGovernor(0)
	payload := []byte("hello world, unthrottled")
	rc := nopReadCloser{bytes.NewReader(payload)}
	wrapped := g.Wrap(context.Background(), rc)
	got, err := io.ReadAll(wrapped)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	if !bytes.Equal(got, payload) {
		t.Fatalf("got %q, want %q", got, payload)
	}
}

func TestGovernorThrottles(t *testing.T) {
	// 100 KB/s. Reading 50 KB should take at least ~0.4s (after the initial burst
	// budget is consumed). We assert a conservative lower bound to avoid flakiness.
	const kbps = 100
	const total = 50 * 1024
	g := NewSpeedGovernor(kbps)

	src := nopReadCloser{bytes.NewReader(make([]byte, total))}
	wrapped := g.Wrap(context.Background(), src)

	start := time.Now()
	n, err := io.Copy(io.Discard, wrapped)
	elapsed := time.Since(start)
	if err != nil {
		t.Fatalf("copy: %v", err)
	}
	if n != total {
		t.Fatalf("copied %d, want %d", n, total)
	}
	// With a burst of one second's worth of bytes (100 KB) >= total (50 KB), the
	// throttle may admit it quickly; so use a smaller-than-burst-independent check:
	// reading MORE than the burst must take measurable time. Re-run with 200 KB.
	_ = elapsed

	g2 := NewSpeedGovernor(kbps)
	const big = 250 * 1024 // > burst (100 KB) so the limiter must wait
	src2 := nopReadCloser{bytes.NewReader(make([]byte, big))}
	wrapped2 := g2.Wrap(context.Background(), src2)
	start2 := time.Now()
	if _, err := io.Copy(io.Discard, wrapped2); err != nil {
		t.Fatalf("copy2: %v", err)
	}
	elapsed2 := time.Since(start2)
	// 250 KB at 100 KB/s with a 100 KB burst => ~1.5s of waiting. Assert >= 1s to
	// be safe against scheduler jitter.
	if elapsed2 < 1*time.Second {
		t.Fatalf("expected throttling to take >= 1s for %d bytes at %d KB/s, took %v", big, kbps, elapsed2)
	}
}

func TestGovernorSetLimitMidStreamAppliesLive(t *testing.T) {
	// Start unlimited, then drop to a tight limit mid-stream and confirm the later
	// reads slow down (the reader loads the current limiter each Read).
	g := NewSpeedGovernor(0)
	const total = 300 * 1024
	src := nopReadCloser{bytes.NewReader(make([]byte, total))}
	wrapped := g.Wrap(context.Background(), src)

	buf := make([]byte, 32*1024)
	// Read the first chunk unlimited.
	if _, err := wrapped.Read(buf); err != nil && err != io.EOF {
		t.Fatalf("first read: %v", err)
	}

	// Now clamp to 50 KB/s and time the remaining reads.
	g.SetLimitKBps(50)
	start := time.Now()
	for {
		_, err := wrapped.Read(buf)
		if err == io.EOF {
			break
		}
		if err != nil {
			t.Fatalf("read: %v", err)
		}
	}
	elapsed := time.Since(start)
	// Remaining ~268 KB at 50 KB/s (burst 50 KB) => well over 1s of waiting.
	if elapsed < 1*time.Second {
		t.Fatalf("expected live limit change to throttle remaining reads, took %v", elapsed)
	}
}

func TestGovernorSetLimitToUnlimited(t *testing.T) {
	g := NewSpeedGovernor(50)
	g.SetLimitKBps(0) // back to unlimited
	if g.Limiter() != nil {
		t.Fatalf("after SetLimitKBps(0) the governor's limiter should be nil (unlimited)")
	}
	if g.LimitKBps() != 0 {
		t.Fatalf("LimitKBps() = %d, want 0 after going unlimited", g.LimitKBps())
	}
}
