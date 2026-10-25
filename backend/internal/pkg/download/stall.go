package download

import (
	"errors"
	"io"
	"sync/atomic"
	"time"
)

// defaultStallTimeout is how long a response body may deliver no bytes before the
// attempt is aborted and retried (Range-resumed). Throttled or DPI-filtered links
// often stay open with zero bytes forever; the transport only bounds the headers.
const defaultStallTimeout = 30 * time.Second

// errStalled is retryable: the retry loop reconnects and resumes at the offset.
var errStalled = errors.New("stalled, reconnecting")

// stallTimeout resolves Options.StallTimeout: 0 -> default, negative -> disabled.
func (o Options) stallTimeout() time.Duration {
	switch {
	case o.StallTimeout < 0:
		return 0
	case o.StallTimeout == 0:
		return defaultStallTimeout
	}
	return o.StallTimeout
}

// idleReader aborts a body read that blocks longer than timeout. It must wrap the
// RAW network body (inside any rate limiter) so only time blocked in the network
// Read counts; limiter waits happen outside it. A time.AfterFunc closes the body
// to unblock the Read, so no goroutine is left behind.
type idleReader struct {
	rc      io.ReadCloser
	timeout time.Duration
	stalled atomic.Bool
}

func newIdleReader(rc io.ReadCloser, timeout time.Duration) io.ReadCloser {
	if timeout <= 0 {
		return rc
	}
	return &idleReader{rc: rc, timeout: timeout}
}

func (r *idleReader) Read(p []byte) (int, error) {
	t := time.AfterFunc(r.timeout, func() {
		r.stalled.Store(true)
		r.rc.Close()
	})
	n, err := r.rc.Read(p)
	t.Stop()
	if r.stalled.Load() {
		if n > 0 {
			return n, nil // deliver bytes; the next Read reports the stall
		}
		return 0, errStalled
	}
	return n, err
}

func (r *idleReader) Close() error { return r.rc.Close() }
