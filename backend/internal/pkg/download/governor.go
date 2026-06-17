package download

import (
	"context"
	"io"
	"sync/atomic"

	"golang.org/x/time/rate"
)

// SpeedGovernor is a process-wide, live-adjustable bandwidth cap shared across
// concurrent downloads. Unlike the per-download rate.Limiter built by
// newSpeedLimiter, a single governor enforces ONE global budget: all readers
// wrapped by the same governor draw from the same limiter, so N concurrent
// downloads together stay under the cap (not N times it).
//
// The current limiter is stored in an atomic pointer (nil = unlimited). The
// reader returned by Wrap loads the pointer on every Read, so a SetLimitKBps
// call applies to in-flight downloads immediately — no restart needed. This is
// what the schedule controller uses to open/close bandwidth windows on the fly.
//
// SpeedGovernor is safe for concurrent use.
type SpeedGovernor struct {
	// limiter holds a *rate.Limiter; a nil value means "unlimited".
	limiter atomic.Pointer[rate.Limiter]
}

// NewSpeedGovernor returns a governor capped at kbps kilobytes per second. A
// kbps <= 0 means unlimited.
func NewSpeedGovernor(kbps int) *SpeedGovernor {
	g := &SpeedGovernor{}
	g.SetLimitKBps(kbps)
	return g
}

// SetLimitKBps changes the cap live. kbps <= 0 switches the governor to
// unlimited; otherwise it installs a limiter at kbps KB/s (reusing the same
// bytes/sec + burst math as the per-download limiter so a single buffer Read is
// always admissible). Existing wrapped readers pick up the change on their next
// Read.
func (g *SpeedGovernor) SetLimitKBps(kbps int) {
	bps := speedLimitBytesPerSec(kbps)
	if bps <= 0 {
		g.limiter.Store(nil)
		return
	}
	burst := bps
	if burst < downloadBufferSize {
		burst = downloadBufferSize
	}
	g.limiter.Store(rate.NewLimiter(rate.Limit(bps), int(burst)))
}

// Limiter returns the current *rate.Limiter, or nil when unlimited.
func (g *SpeedGovernor) Limiter() *rate.Limiter {
	return g.limiter.Load()
}

// LimitKBps reports the current cap in KB/s (0 = unlimited). Intended for
// observability/tests.
func (g *SpeedGovernor) LimitKBps() int {
	l := g.limiter.Load()
	if l == nil {
		return 0
	}
	return int(float64(l.Limit()) / bytesPerKB)
}

// Wrap returns a reader that throttles rc to the governor's current cap.
//
// A nil governor returns rc unchanged (the engine/CLI fallback path: callers
// that pass no governor are never throttled here). A real governor ALWAYS
// returns a wrapping reader — even when currently unlimited — because the
// wrapper re-reads the governor's limiter on every Read, so a window that opens
// mid-download (SetLimitKBps from 0 to a real cap) takes effect immediately. The
// wrapper is a no-op while the limiter is nil, so unlimited periods cost only an
// atomic load per Read.
func (g *SpeedGovernor) Wrap(ctx context.Context, rc io.ReadCloser) io.ReadCloser {
	if g == nil {
		return rc
	}
	return &governedReader{gov: g, reader: rc, ctx: ctx}
}

// governedReader throttles reads using whatever limiter the governor currently
// holds. If the governor becomes unlimited mid-stream (limiter == nil) it stops
// waiting and passes bytes through.
type governedReader struct {
	gov    *SpeedGovernor
	reader io.ReadCloser
	ctx    context.Context
}

func (r *governedReader) Read(p []byte) (int, error) {
	n, err := r.reader.Read(p)
	if n > 0 {
		if l := r.gov.limiter.Load(); l != nil {
			if waitErr := l.WaitN(r.ctx, n); waitErr != nil {
				return n, waitErr
			}
		}
	}
	return n, err
}

func (r *governedReader) Close() error {
	return r.reader.Close()
}
