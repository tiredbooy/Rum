package download

import (
	"crypto/tls"
	"errors"
	"net/http"
	"net/url"
	"testing"
)

func TestValidateURL(t *testing.T) {
	tests := []struct {
		name    string
		raw     string
		wantErr error // nil, ErrInvalidURL, or ErrUnsupportedScheme
	}{
		{"plain http", "http://example.com/file.zip", nil},
		{"plain https", "https://example.com/file.zip", nil},
		{"https with port", "https://example.com:8443/a", nil},
		{"uppercase scheme", "HTTPS://example.com/a", nil},
		{"https no path", "https://example.com", nil},
		{"empty", "", ErrInvalidURL},
		{"whitespace only", "   ", ErrInvalidURL},
		{"missing scheme", "example.com/file", ErrInvalidURL},
		{"missing host", "http:///file", ErrInvalidURL},
		{"opaque mailto", "mailto:user@example.com", ErrUnsupportedScheme},
		{"ftp scheme", "ftp://example.com/file", ErrUnsupportedScheme},
		{"file scheme", "file:///etc/passwd", ErrUnsupportedScheme},
		{"data scheme", "data:text/plain,hello", ErrUnsupportedScheme},
		{"javascript scheme", "javascript:alert(1)", ErrUnsupportedScheme},
		{"control chars", "http://exa\x7fmple.com", ErrInvalidURL},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateURL(tt.raw)
			if tt.wantErr == nil {
				if err != nil {
					t.Fatalf("ValidateURL(%q) = %v, want nil", tt.raw, err)
				}
				return
			}
			if err == nil {
				t.Fatalf("ValidateURL(%q) = nil, want %v", tt.raw, tt.wantErr)
			}
			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("ValidateURL(%q) = %v, want wrap of %v", tt.raw, err, tt.wantErr)
			}
		})
	}
}

func TestSecureTransport_NilBase(t *testing.T) {
	tr := SecureTransport(nil)
	if tr == nil {
		t.Fatal("SecureTransport(nil) returned nil")
	}
	if tr.TLSClientConfig == nil {
		t.Fatal("TLSClientConfig is nil")
	}
	if tr.TLSClientConfig.MinVersion != tls.VersionTLS12 {
		t.Fatalf("MinVersion = %x, want %x", tr.TLSClientConfig.MinVersion, tls.VersionTLS12)
	}
	if !tr.ForceAttemptHTTP2 {
		t.Fatal("ForceAttemptHTTP2 = false, want true")
	}
	if tr.TLSClientConfig.InsecureSkipVerify {
		t.Fatal("InsecureSkipVerify = true, must never be set")
	}
	if tr.ResponseHeaderTimeout == 0 || tr.ExpectContinueTimeout == 0 {
		t.Fatal("timeouts not set")
	}
}

func TestSecureTransport_PreservesBase(t *testing.T) {
	base := &http.Transport{
		MaxConnsPerHost: 7,
	}
	base.TLSClientConfig = &tls.Config{ServerName: "custom"}

	tr := SecureTransport(base)
	if tr == base {
		t.Fatal("SecureTransport must not return the same pointer (no mutation of base)")
	}
	if tr.MaxConnsPerHost != 7 {
		t.Fatalf("MaxConnsPerHost = %d, want 7 (preserved)", tr.MaxConnsPerHost)
	}
	if tr.TLSClientConfig.ServerName != "custom" {
		t.Fatalf("ServerName = %q, want preserved", tr.TLSClientConfig.ServerName)
	}
	if tr.TLSClientConfig.MinVersion != tls.VersionTLS12 {
		t.Fatalf("MinVersion = %x, want clamped to TLS1.2", tr.TLSClientConfig.MinVersion)
	}
	// base must be untouched.
	if base.ForceAttemptHTTP2 {
		t.Fatal("base.ForceAttemptHTTP2 mutated")
	}
	if base.TLSClientConfig.MinVersion == tls.VersionTLS12 {
		t.Fatal("base.TLSClientConfig was mutated (MinVersion changed)")
	}
}

func TestSecureTransport_DoesNotLowerMinVersion(t *testing.T) {
	base := &http.Transport{TLSClientConfig: &tls.Config{MinVersion: tls.VersionTLS13}}
	tr := SecureTransport(base)
	if tr.TLSClientConfig.MinVersion != tls.VersionTLS13 {
		t.Fatalf("MinVersion = %x, want TLS1.3 (must not be lowered)", tr.TLSClientConfig.MinVersion)
	}
}

func mustReq(t *testing.T, rawurl string, hdr map[string]string) *http.Request {
	t.Helper()
	u, err := url.Parse(rawurl)
	if err != nil {
		t.Fatalf("parse %q: %v", rawurl, err)
	}
	r := &http.Request{URL: u, Header: http.Header{}}
	for k, v := range hdr {
		r.Header.Set(k, v)
	}
	return r
}

func TestRedirectPolicy_Cap(t *testing.T) {
	req := mustReq(t, "https://a.com/x", nil)

	// 9 prior redirects -> still allowed.
	via9 := make([]*http.Request, 9)
	for i := range via9 {
		via9[i] = mustReq(t, "https://a.com/x", nil)
	}
	if err := RedirectPolicy(req, via9); err != nil {
		t.Fatalf("9 redirects should be allowed, got %v", err)
	}

	// 10 prior redirects -> rejected.
	via10 := make([]*http.Request, 10)
	for i := range via10 {
		via10[i] = mustReq(t, "https://a.com/x", nil)
	}
	if err := RedirectPolicy(req, via10); err == nil {
		t.Fatal("10 redirects should be rejected, got nil")
	}
}

func TestRedirectPolicy_StripsSensitiveOnCrossHost(t *testing.T) {
	req := mustReq(t, "https://evil.com/x", map[string]string{
		"Authorization": "Bearer secret",
		"Cookie":        "session=abc",
		"User-Agent":    "Rum",
	})
	via := []*http.Request{mustReq(t, "https://trusted.com/orig", nil)}

	if err := RedirectPolicy(req, via); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got := req.Header.Get("Authorization"); got != "" {
		t.Fatalf("Authorization not stripped on cross-host: %q", got)
	}
	if got := req.Header.Get("Cookie"); got != "" {
		t.Fatalf("Cookie not stripped on cross-host: %q", got)
	}
	if got := req.Header.Get("User-Agent"); got != "Rum" {
		t.Fatalf("non-sensitive header dropped: %q", got)
	}
}

func TestRedirectPolicy_KeepsSensitiveOnSameHost(t *testing.T) {
	req := mustReq(t, "https://same.com/new", map[string]string{
		"Authorization": "Bearer secret",
		"Cookie":        "session=abc",
	})
	via := []*http.Request{mustReq(t, "https://same.com/old", nil)}

	if err := RedirectPolicy(req, via); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if req.Header.Get("Authorization") != "Bearer secret" {
		t.Fatal("Authorization wrongly stripped on same-host redirect")
	}
	if req.Header.Get("Cookie") != "session=abc" {
		t.Fatal("Cookie wrongly stripped on same-host redirect")
	}
}

func TestRedirectPolicy_SameHostDifferentPort(t *testing.T) {
	// Different port, same hostname -> credentials kept (hostname scoping).
	req := mustReq(t, "https://same.com:8443/new", map[string]string{
		"Authorization": "Bearer secret",
	})
	via := []*http.Request{mustReq(t, "https://same.com/old", nil)}
	if err := RedirectPolicy(req, via); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if req.Header.Get("Authorization") != "Bearer secret" {
		t.Fatal("Authorization wrongly stripped across ports on same host")
	}
}

func TestRedirectPolicy_AssignableToCheckRedirect(t *testing.T) {
	// Compile-time guarantee the signature matches http.Client.CheckRedirect.
	c := &http.Client{CheckRedirect: RedirectPolicy}
	if c.CheckRedirect == nil {
		t.Fatal("CheckRedirect not assigned")
	}
}

func TestIsPrivateHost(t *testing.T) {
	tests := []struct {
		host string
		want bool
	}{
		{"127.0.0.1", true},
		{"::1", true},
		{"10.0.0.5", true},
		{"192.168.1.1", true},
		{"172.16.0.1", true},
		{"169.254.0.1", true}, // link-local
		{"0.0.0.0", true},     // unspecified
		{"100.64.0.1", true},  // CGNAT
		{"100.127.255.255", true},
		{"fc00::1", true}, // unique local
		{"8.8.8.8", false},
		{"1.1.1.1", false},
		{"93.184.216.34", false},
		{"100.63.255.255", false}, // just below CGNAT
		{"100.128.0.1", false},    // just above CGNAT
		{"example.com", false},    // DNS name, not an IP literal
		{"", false},
	}
	for _, tt := range tests {
		t.Run(tt.host, func(t *testing.T) {
			if got := IsPrivateHost(tt.host); got != tt.want {
				t.Fatalf("IsPrivateHost(%q) = %v, want %v", tt.host, got, tt.want)
			}
		})
	}
}
