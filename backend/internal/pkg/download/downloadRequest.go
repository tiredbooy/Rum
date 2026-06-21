package download

import (
	"context"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"strings"
	"time"

	"github.com/tiredbooy/Rum/backend/internal/pkg/config"
)

// maxConnsPerHost bounds the number of TCP connections the shared client keeps
// open to a single host. Segmented downloads open several connections to the
// same host concurrently, so the old value of 2 throttled throughput badly.
const maxConnsPerHost = 16

type HeaderInfo struct {
	ContentSize        string
	ContentType        string
	ContentDisposition string
	SupportsRange      bool
}

type Downloader struct {
	Client  *http.Client
	Headers map[string]string
}

type RequestHeaders struct {
	Referer        string
	UserAgent      string
	AcceptLanguage string
	AcceptEncoding string
	Connection     string
}

func NewDownloader(userAgent, referer string) *Downloader {
	jar, _ := cookiejar.New(nil)
	baseTransport := &http.Transport{
		DialContext: (&net.Dialer{
			Timeout:   30 * time.Second,
			KeepAlive: 30 * time.Second,
		}).DialContext,
		MaxConnsPerHost:     maxConnsPerHost,
		TLSHandshakeTimeout: 10 * time.Second,
	}
	// Route outbound downloads through the user-configured proxy, if any. This is
	// preserved by SecureTransport's base.Clone() below. Without this the Proxy
	// setting was persisted and shown in the UI but never applied to any
	// transport — a dead feature (and a no-op for the Iran/Persian audience where
	// a proxy is usually required).
	baseTransport.Proxy = proxyTransportFunc()
	client := &http.Client{
		Jar: jar,
		// SecureTransport enforces a TLS 1.2 floor and stall timeouts;
		// RedirectPolicy caps the redirect chain and strips credentials on
		// cross-host redirects (both defined in security.go).
		Transport:     SecureTransport(baseTransport),
		CheckRedirect: RedirectPolicy,
	}

	headers := map[string]string{
		"User-Agent": userAgent,
		// "Referer":         referer,
		"Accept":          "*/*",
		"Accept-Language": "en-US,en;q=0.5",
		// NOTE: deliberately do NOT set Accept-Encoding here. When a request
		// sets Accept-Encoding manually, Go's stdlib disables its transparent
		// decompression, which makes Content-Length and resume byte offsets
		// unreliable (the body is then compressed and offsets no longer map to
		// file positions). Leaving it unset lets net/http negotiate gzip and
		// decompress transparently, so byte offsets are always raw file bytes.
		"Connection": "keep-alive",
	}

	if referer != "" {
		headers["Referer"] = referer
	}

	return &Downloader{
		Client:  client,
		Headers: headers,
	}
}

// proxyTransportFunc returns an http.Transport.Proxy function for the
// user-configured proxy, or nil when no (valid) proxy is set. net/http natively
// dials http, https, and socks5/socks5h proxy URLs, so a single ProxyURL covers
// all three. A bare "host:port" is treated as an http proxy. An invalid or
// unsupported value is ignored (logged) rather than breaking every download.
//
// The proxy is read once when a Downloader is built; downloads started after a
// settings change pick up the new value because the engine builds a fresh
// Downloader on the create/repair paths.
func proxyTransportFunc() func(*http.Request) (*url.URL, error) {
	var setting config.Setting
	if err := setting.LoadSettingMetadata(); err != nil {
		return nil
	}
	raw := strings.TrimSpace(setting.Proxy)
	if raw == "" {
		return nil
	}
	if !strings.Contains(raw, "://") {
		raw = "http://" + raw // bare host:port -> http proxy
	}
	u, err := url.Parse(raw)
	if err != nil || u.Host == "" || !isSupportedProxyScheme(u.Scheme) {
		log.Printf("download: ignoring invalid proxy %q", setting.Proxy)
		return nil
	}
	return http.ProxyURL(u)
}

func isSupportedProxyScheme(scheme string) bool {
	switch strings.ToLower(scheme) {
	case "http", "https", "socks5", "socks5h":
		return true
	default:
		return false
	}
}

func (d *Downloader) NewRequest(method, urlStr string) (*http.Request, error) {
	req, err := http.NewRequest(method, urlStr, nil)
	if err != nil {
		return nil, err
	}
	for key, value := range d.Headers {
		req.Header.Set(key, value)
	}

	if req.Header.Get("Referer") == "" {
		u, err := url.Parse(urlStr)
		if err == nil {
			req.Header.Set("Referer", fmt.Sprintf("%s://%s/", u.Scheme, u.Host))
		}
	}

	return req, nil
}

func (d *Downloader) GetFileRequest(url string) (*http.Request, error) {
	return d.NewRequest("GET", url)
}

func GetHeader(url string) (*http.Response, error) {
	req, err := http.NewRequest("HEAD", url, nil)
	if err != nil {
		log.Printf("Failed to create HEAD request for %s: %v", url, err)
		return nil, err
	}

	client := GetHTTPClient(15 * time.Second)
	return client.Do(req)
}

func (d *Downloader) HeadWithFallback(url string) (*HeaderInfo, error) {
	return d.HeadWithFallbackContext(context.Background(), url)
}

// HeadWithFallbackContext probes a URL for size/range/content metadata, honoring
// the provided context. Because the request itself is bound to ctx, cancelling
// or timing out ctx makes the in-flight HTTP call return promptly instead of
// leaking a goroutine that blocks on the network indefinitely.
func (d *Downloader) HeadWithFallbackContext(ctx context.Context, url string) (*HeaderInfo, error) {
	// Try HEAD first
	headReq, _ := d.NewRequest("HEAD", url)
	resp, err := d.Client.Do(headReq.WithContext(ctx))
	if err == nil && resp.StatusCode < 400 {
		defer resp.Body.Close()
		return ParseHeaderInfo(resp), nil
	}
	if resp != nil {
		resp.Body.Close()
	}
	if ctx.Err() != nil {
		return &HeaderInfo{}, ctx.Err()
	}

	// Fallback: GET with Range: bytes=0-0
	req, _ := d.NewRequest("GET", url)
	req.Header.Set("Range", "bytes=0-0")
	resp, err = d.Client.Do(req.WithContext(ctx))
	if err == nil {
		defer resp.Body.Close()
		// Drain a small body so the connection can be reused.
		io.Copy(io.Discard, io.LimitReader(resp.Body, 1<<10))
		return ParseHeaderInfo(resp), nil
	}

	if ctx.Err() != nil {
		return &HeaderInfo{}, ctx.Err()
	}
	return &HeaderInfo{}, nil
}

func GetHeaderInfo(url string) (*HeaderInfo, error) {
	headResp, err := GetWithTimeout(url, "HEAD", 10*time.Second)
	if err == nil && headResp != nil {
		defer headResp.Body.Close()
		return ParseHeaderInfo(headResp), nil
	}

	req, _ := http.NewRequest("GET", url, nil)
	req.Header.Set("Range", "bytes=0.0")

	client := GetHTTPClient(15 * time.Second)
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("HEAD and GET fallback both failed: %s", err)
	}

	defer resp.Body.Close()

	io.Copy(io.Discard, resp.Body)

	return ParseHeaderInfo(resp), nil
}

func GetHTTPClient(timeout time.Duration) *http.Client {
	jar, _ := cookiejar.New(nil)
	return &http.Client{
		Timeout: timeout,
		Jar:     jar,
		Transport: &http.Transport{
			DialContext: (&net.Dialer{
				Timeout:   30 * time.Second,
				KeepAlive: 30 * time.Second,
				DualStack: true,
			}).DialContext,
			MaxConnsPerHost:     maxConnsPerHost,
			TLSHandshakeTimeout: 10 * time.Second,
		},
	}
}

func ParseHeaderInfo(resp *http.Response) *HeaderInfo {
	acceptRange := resp.Header.Get("Accept-Ranges")
	// A 206 Partial Content response (e.g. to our "bytes=0-0" probe) is itself
	// proof the server honors Range requests, even if it omits Accept-Ranges.
	supportsRange := (acceptRange != "" && acceptRange != "none") ||
		resp.StatusCode == http.StatusPartialContent

	contentSize := resp.Header.Get("Content-Length")
	// When the probe used Range, Content-Length describes only the returned
	// slice (1 byte). The real total lives in Content-Range: "bytes 0-0/12345".
	if cr := resp.Header.Get("Content-Range"); cr != "" {
		if idx := strings.LastIndex(cr, "/"); idx != -1 {
			if total := strings.TrimSpace(cr[idx+1:]); total != "" && total != "*" {
				contentSize = total
			}
		}
	}

	return &HeaderInfo{
		ContentSize:        contentSize,
		ContentType:        resp.Header.Get("Content-Type"),
		ContentDisposition: resp.Header.Get("Content-Disposition"),
		SupportsRange:      supportsRange,
	}
}

func GetWithTimeout(url, method string, timeout time.Duration) (*http.Response, error) {
	req, _ := http.NewRequest(method, url, nil)
	client := &http.Client{Timeout: timeout}
	return client.Do(req)
}
