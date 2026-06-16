package download

import (
	"crypto/tls"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// maxRedirects bounds how many redirects the shared HTTP client will follow.
// Beyond this RedirectPolicy returns an error, mirroring the stdlib default of
// 10 but making the limit explicit and overridable in one place.
const maxRedirects = 10

// sensitiveRedirectHeaders are stripped from a request when a redirect crosses
// to a different host, so credentials minted for host A are never replayed to
// host B. This is the same precaution the Go stdlib applies for Authorization,
// extended here to Cookie as well.
var sensitiveRedirectHeaders = []string{"Authorization", "Cookie"}

// ValidateURL parses raw and enforces the engine's URL policy: it must be a
// well-formed absolute URL with an http or https scheme and a non-empty host.
//
// Malformed, empty, or opaque (non-hierarchical, e.g. "mailto:x") inputs wrap
// ErrInvalidURL. A syntactically valid URL whose scheme is not http/https wraps
// ErrUnsupportedScheme. On success it returns nil.
func ValidateURL(raw string) error {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return fmt.Errorf("empty url: %w", ErrInvalidURL)
	}

	u, err := url.Parse(trimmed)
	if err != nil {
		return fmt.Errorf("parse url %q: %v: %w", raw, err, ErrInvalidURL)
	}

	// Scheme is checked before the opaque/host checks: an explicit but
	// unsupported scheme (mailto, data, javascript, ftp, file...) should report
	// ErrUnsupportedScheme even when the URL is opaque, since the scheme is the
	// salient reason it was rejected.
	scheme := strings.ToLower(u.Scheme)
	switch scheme {
	case "http", "https":
		// allowed
	case "":
		// No scheme means it was parsed as a relative reference; there is no
		// host to download from.
		return fmt.Errorf("url %q missing scheme: %w", raw, ErrInvalidURL)
	default:
		return fmt.Errorf("scheme %q not allowed: %w", scheme, ErrUnsupportedScheme)
	}

	// Opaque URLs (u.Opaque != "") are non-hierarchical like "http:foo"; with an
	// http/https scheme they carry no host and are never valid download targets.
	if u.Opaque != "" {
		return fmt.Errorf("opaque url %q: %w", raw, ErrInvalidURL)
	}

	if u.Hostname() == "" {
		return fmt.Errorf("url %q missing host: %w", raw, ErrInvalidURL)
	}

	return nil
}

// SecureTransport returns an *http.Transport hardened for outbound downloads.
//
// It clones base (or builds a fresh transport when base is nil) and then:
//   - enforces TLS >= 1.2 via TLSClientConfig.MinVersion (cert verification is
//     left ON; InsecureSkipVerify is never set),
//   - enables ForceAttemptHTTP2 so HTTP/2 is negotiated when available,
//   - sets a ResponseHeaderTimeout and ExpectContinueTimeout so a stalled
//     server cannot hang a download forever,
//   - preserves the caller's DialContext, MaxConnsPerHost, TLSHandshakeTimeout
//     and all other tuning already present on base.
//
// The returned transport is a copy; base is not mutated.
func SecureTransport(base *http.Transport) *http.Transport {
	var t *http.Transport
	if base != nil {
		t = base.Clone()
	} else {
		t = &http.Transport{
			DialContext: (&net.Dialer{
				Timeout:   30 * time.Second,
				KeepAlive: 30 * time.Second,
				DualStack: true,
			}).DialContext,
			MaxConnsPerHost:     maxConnsPerHost,
			TLSHandshakeTimeout: 10 * time.Second,
		}
	}

	// Preserve any TLS settings the caller configured (e.g. custom RootCAs)
	// while clamping the minimum protocol version. Never weaken verification.
	if t.TLSClientConfig == nil {
		t.TLSClientConfig = &tls.Config{}
	} else {
		t.TLSClientConfig = t.TLSClientConfig.Clone()
	}
	if t.TLSClientConfig.MinVersion < tls.VersionTLS12 {
		t.TLSClientConfig.MinVersion = tls.VersionTLS12
	}

	t.ForceAttemptHTTP2 = true

	if t.ResponseHeaderTimeout == 0 {
		t.ResponseHeaderTimeout = 30 * time.Second
	}
	if t.ExpectContinueTimeout == 0 {
		t.ExpectContinueTimeout = 1 * time.Second
	}
	if t.IdleConnTimeout == 0 {
		t.IdleConnTimeout = 90 * time.Second
	}

	return t
}

// RedirectPolicy is assignable to http.Client.CheckRedirect. It caps the
// redirect chain at maxRedirects and, on any redirect that lands on a different
// host than the previous request, strips sensitive headers (Authorization,
// Cookie) from req so credentials are not leaked across hosts.
func RedirectPolicy(req *http.Request, via []*http.Request) error {
	if len(via) >= maxRedirects {
		return fmt.Errorf("stopped after %d redirects", maxRedirects)
	}

	if len(via) > 0 {
		prev := via[len(via)-1]
		if prev.URL != nil && req.URL != nil &&
			!sameHost(prev.URL.Hostname(), req.URL.Hostname()) {
			for _, h := range sensitiveRedirectHeaders {
				req.Header.Del(h)
			}
		}
	}

	return nil
}

// sameHost compares two hostnames case-insensitively. Hosts are compared by
// hostname only (port is irrelevant for credential scoping here).
func sameHost(a, b string) bool {
	return strings.EqualFold(a, b)
}

// IsPrivateHost reports whether host (a hostname or IP literal) refers to a
// loopback, private, link-local, or otherwise non-public address.
//
// It only inspects IP literals; a DNS name is treated as non-private (returns
// false) because resolving it here would be both slow and racy against the
// actual dial. Callers wanting a real SSRF guard should resolve and check each
// dialed IP (see the dial-time hook described on BlockPrivateHosts).
//
// NOTE: This is provided for an OPT-IN SSRF guard only. A download manager
// legitimately fetches from arbitrary hosts, so private-host blocking is OFF by
// default and must be explicitly enabled by the orchestrator.
func IsPrivateHost(host string) bool {
	host = strings.TrimSpace(host)
	if host == "" {
		return false
	}
	ip := net.ParseIP(host)
	if ip == nil {
		return false
	}
	return isPrivateIP(ip)
}

func isPrivateIP(ip net.IP) bool {
	if ip.IsLoopback() || ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast() ||
		ip.IsPrivate() || ip.IsUnspecified() {
		return true
	}
	// Carrier-grade NAT range 100.64.0.0/10 is not covered by IsPrivate.
	if v4 := ip.To4(); v4 != nil {
		if v4[0] == 100 && v4[1] >= 64 && v4[1] <= 127 {
			return true
		}
	}
	return false
}
