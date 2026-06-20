package download

import (
	"context"
	"errors"
	"io"
	"math"
	"math/rand"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// httpStatusError carries an HTTP status code so retry logic can distinguish
// permanent client errors (4xx) from transient server/network errors.
type httpStatusError struct {
	code int
}

func (e *httpStatusError) Error() string {
	return "unexpected status: " + http.StatusText(e.code)
}

// retryDefaults
const (
	defaultRetryBaseDelay = 500 * time.Millisecond
	defaultRetryMaxDelay  = 30 * time.Second
)

// isRetryableError reports whether err represents a transient failure worth
// retrying (network resets, timeouts, EOFs, 5xx / 429 responses) as opposed to
// a permanent one (context cancellation, HTTP 4xx other than 429, malformed
// requests). Context cancellation is never retried — pausing must stop work.
func isRetryableError(err error) bool {
	if err == nil {
		return false
	}
	// Never retry on cancellation / deadline from the caller: that is an
	// intentional pause/stop, not a transient fault.
	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return false
	}

	// A changed remote validator (E1) must never be retried: resuming would only
	// re-staple new-version bytes onto old ones. The segmented driver catches this
	// sentinel and restarts the whole download from zero instead.
	if errors.Is(err, errValidatorChanged) {
		return false
	}

	// HTTP status based decisions.
	var se *httpStatusError
	if errors.As(err, &se) {
		switch {
		case se.code == http.StatusTooManyRequests: // 429
			return true
		case se.code == http.StatusRequestTimeout: // 408
			return true
		case se.code >= 500 && se.code <= 599: // server errors
			return true
		case se.code >= 400 && se.code <= 499: // other client errors -> permanent
			return false
		default:
			return false
		}
	}

	// Connection-level transient conditions.
	if errors.Is(err, io.ErrUnexpectedEOF) || errors.Is(err, io.EOF) {
		return true
	}

	var netErr net.Error
	if errors.As(err, &netErr) {
		// Timeouts and most net errors are transient.
		return true
	}

	var urlErr *url.Error
	if errors.As(err, &urlErr) {
		return isRetryableError(urlErr.Err)
	}

	// Heuristic fallback for wrapped connection-reset style errors.
	msg := strings.ToLower(err.Error())
	for _, s := range []string{"connection reset", "broken pipe", "connection refused",
		"unexpected eof", "timeout", "temporarily unavailable", "no route to host"} {
		if strings.Contains(msg, s) {
			return true
		}
	}
	return false
}

// backoffDelay computes the exponential backoff with full jitter for a given
// attempt (0-based): base*2^attempt, capped at maxDelay, then randomized in
// [0, computed]. rng may be nil to use the package default source.
func backoffDelay(attempt int, base, maxDelay time.Duration, rng *rand.Rand) time.Duration {
	if base <= 0 {
		base = defaultRetryBaseDelay
	}
	if maxDelay <= 0 {
		maxDelay = defaultRetryMaxDelay
	}
	// base * 2^attempt with overflow protection.
	exp := float64(base) * math.Pow(2, float64(attempt))
	if exp > float64(maxDelay) || math.IsInf(exp, 1) {
		exp = float64(maxDelay)
	}
	d := time.Duration(exp)
	if d <= 0 {
		d = base
	}
	// Full jitter: sleep a random amount in [0, d].
	if rng != nil {
		return time.Duration(rng.Int63n(int64(d) + 1))
	}
	return time.Duration(rand.Int63n(int64(d) + 1))
}

// sleepWithContext sleeps for d or until ctx is cancelled, whichever first.
// Returns ctx.Err() if cancelled.
func sleepWithContext(ctx context.Context, d time.Duration) error {
	if d <= 0 {
		return ctx.Err()
	}
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-t.C:
		return nil
	}
}

// retryConfig holds the parameters for retryWithBackoff.
type retryConfig struct {
	maxRetries int
	base       time.Duration
	maxDelay   time.Duration
}

// newRetryConfig builds a retry config from opt. opt.MaxRetries bounds the retry
// count; opt.RetryBackoffSec (seconds) overrides the base exponential-backoff
// delay when > 0, otherwise the engine default (defaultRetryBaseDelay) is used.
// Threading the base lets the AutoResume/retry settings tune how aggressively a
// transient (e.g. reconnect) failure is retried before surfacing.
func newRetryConfig(opt Options) retryConfig {
	maxRetries := opt.MaxRetries
	if maxRetries < 0 {
		maxRetries = 0
	}
	base := defaultRetryBaseDelay
	if opt.RetryBackoffSec > 0 {
		base = time.Duration(opt.RetryBackoffSec) * time.Second
	}
	return retryConfig{
		maxRetries: maxRetries,
		base:       base,
		maxDelay:   defaultRetryMaxDelay,
	}
}

// retryWithBackoff runs fn, retrying on transient errors up to cfg.maxRetries
// times with exponential backoff + jitter. It returns the last error. Context
// cancellation aborts immediately and is never retried.
func retryWithBackoff(ctx context.Context, cfg retryConfig, fn func(ctx context.Context) error) error {
	var lastErr error
	for attempt := 0; ; attempt++ {
		if err := ctx.Err(); err != nil {
			if lastErr != nil {
				return lastErr
			}
			return err
		}

		lastErr = fn(ctx)
		if lastErr == nil {
			return nil
		}
		if attempt >= cfg.maxRetries {
			return lastErr
		}
		if !isRetryableError(lastErr) {
			return lastErr
		}

		delay := backoffDelay(attempt, cfg.base, cfg.maxDelay, nil)
		if err := sleepWithContext(ctx, delay); err != nil {
			// Cancelled while waiting to retry: surface the original error if
			// it was a real fault, else the context error.
			if lastErr != nil {
				return lastErr
			}
			return err
		}
	}
}
