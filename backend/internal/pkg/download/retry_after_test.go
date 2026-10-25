package download

import (
	"context"
	"crypto/tls"
	"errors"
	"net"
	"net/http"
	"net/http/httptest"
	"syscall"
	"testing"
	"time"
)

func TestParseRetryAfter(t *testing.T) {
	now := time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC)
	cases := []struct {
		name string
		in   string
		want time.Duration
	}{
		{"seconds", "7", 7 * time.Second},
		{"padded", " 30 ", 30 * time.Second},
		{"date", now.Add(90 * time.Second).UTC().Format(http.TimeFormat), 90 * time.Second},
		{"past date", now.Add(-time.Minute).UTC().Format(http.TimeFormat), 0},
		{"cap seconds", "99999", maxRetryAfter},
		{"cap date", now.Add(24 * time.Hour).UTC().Format(http.TimeFormat), maxRetryAfter},
		{"negative", "-5", 0},
		{"garbage", "soon", 0},
		{"empty", "", 0},
	}
	for _, c := range cases {
		if got := parseRetryAfter(c.in, now); got != c.want {
			t.Errorf("%s: parseRetryAfter(%q) = %v, want %v", c.name, c.in, got, c.want)
		}
	}
}

// Retry-After only applies to 429/503.
func TestRetryAfterOnlyForThrottleStatuses(t *testing.T) {
	for code, want := range map[int]time.Duration{429: 3 * time.Second, 503: 3 * time.Second, 500: 0, 404: 0} {
		resp := &http.Response{StatusCode: code, Header: http.Header{"Retry-After": {"3"}}}
		if got := newHTTPStatusError(resp).retryAfter; got != want {
			t.Errorf("status %d: retryAfter = %v, want %v", code, got, want)
		}
	}
}

func TestRetryWithBackoffHonorsRetryAfter(t *testing.T) {
	cfg := retryConfig{maxRetries: 1, base: time.Millisecond, maxDelay: time.Millisecond}
	calls := 0
	start := time.Now()
	err := retryWithBackoff(context.Background(), cfg, func(context.Context) error {
		calls++
		if calls == 1 {
			return &httpStatusError{code: 429, retryAfter: 120 * time.Millisecond}
		}
		return nil
	})
	if err != nil || calls != 2 {
		t.Fatalf("err=%v calls=%d, want nil/2", err, calls)
	}
	if el := time.Since(start); el < 110*time.Millisecond {
		t.Fatalf("retried after %v, want to wait for Retry-After (120ms)", el)
	}
}

func TestRetryBudgetResetsOnProgress(t *testing.T) {
	var progress int64
	cfg := retryConfig{maxRetries: 2, base: time.Microsecond, maxDelay: time.Microsecond,
		progress: func() int64 { return progress }}
	calls := 0
	err := retryWithBackoff(context.Background(), cfg, func(context.Context) error {
		calls++
		if calls < 8 {
			progress += 10 // every attempt moved bytes
			return &httpStatusError{code: 503}
		}
		return nil
	})
	if err != nil || calls != 8 {
		t.Fatalf("err=%v calls=%d, want success after 8 calls despite maxRetries=2", err, calls)
	}
}

func TestRetryBudgetStillStopsWithoutProgress(t *testing.T) {
	cfg := retryConfig{maxRetries: 2, base: time.Microsecond, maxDelay: time.Microsecond,
		progress: func() int64 { return 0 }}
	calls := 0
	err := retryWithBackoff(context.Background(), cfg, func(context.Context) error {
		calls++
		return &httpStatusError{code: 503}
	})
	if err == nil || calls != 3 {
		t.Fatalf("err=%v calls=%d, want failure after 3 calls", err, calls)
	}
}

func TestIsRetryableNetworkFailures(t *testing.T) {
	for name, err := range map[string]error{
		"dns":         &net.DNSError{Err: "no such host", Name: "x.test"},
		"dns wrapped": errors.New("Get \"https://x.test\": dial tcp: lookup x.test: no such host"),
		"refused":     &net.OpError{Op: "dial", Err: syscall.ECONNREFUSED},
		"unreachable": &net.OpError{Op: "dial", Err: syscall.ENETUNREACH},
		"hostunreach": syscall.EHOSTUNREACH,
		"tls reset":   errors.New("remote error: tls: handshake failure"),
		"tls eof":     errors.New("tls: first record does not look like a TLS handshake"),
		"conn reset":  syscall.ECONNRESET,
		"proxy":       errors.New("proxyconnect tcp: dial tcp 127.0.0.1:1: connect: connection refused"),
	} {
		if !isRetryableError(err) {
			t.Errorf("%s: want retryable", name)
		}
	}
	if isRetryableError(&tls.CertificateVerificationError{Err: errors.New("x509: certificate has expired")}) {
		t.Error("certificate verification failure must stay permanent")
	}
}

func TestHTTPStatusErrorFromServer(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Retry-After", "2")
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	defer srv.Close()
	resp, err := http.Get(srv.URL)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if got := newHTTPStatusError(resp).retryAfter; got != 2*time.Second {
		t.Fatalf("retryAfter = %v, want 2s", got)
	}
}
