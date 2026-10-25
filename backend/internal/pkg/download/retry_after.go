package download

import (
	"net/http"
	"strconv"
	"strings"
	"time"
)

// maxRetryAfter caps how long a server's Retry-After can park a download. A
// misconfigured or hostile value (hours, days) must not freeze a job silently.
const maxRetryAfter = 5 * time.Minute

// parseRetryAfter parses a Retry-After header value, which is either a number of
// seconds or an HTTP-date, relative to now. It returns 0 for empty, malformed,
// negative or already-elapsed values, and never more than maxRetryAfter.
func parseRetryAfter(v string, now time.Time) time.Duration {
	v = strings.TrimSpace(v)
	if v == "" {
		return 0
	}
	var d time.Duration
	if secs, err := strconv.ParseInt(v, 10, 64); err == nil {
		if secs <= 0 {
			return 0
		}
		if secs > int64(maxRetryAfter/time.Second) {
			return maxRetryAfter
		}
		d = time.Duration(secs) * time.Second
	} else if t, err := http.ParseTime(v); err == nil {
		d = t.Sub(now)
	} else {
		return 0
	}
	if d <= 0 {
		return 0
	}
	if d > maxRetryAfter {
		return maxRetryAfter
	}
	return d
}

// newHTTPStatusError builds the error for a non-success response, capturing
// Retry-After for the throttling statuses (429, 503) where it is meaningful.
func newHTTPStatusError(resp *http.Response) *httpStatusError {
	e := &httpStatusError{code: resp.StatusCode}
	if resp.StatusCode == http.StatusTooManyRequests || resp.StatusCode == http.StatusServiceUnavailable {
		e.retryAfter = parseRetryAfter(resp.Header.Get("Retry-After"), time.Now())
	}
	return e
}
