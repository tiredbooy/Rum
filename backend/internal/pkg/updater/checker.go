package updater

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/tiredbooy/Rum/backend/internal/pkg/config"
)

// APIURL is where the newest release is read from.
const APIURL = "https://api.github.com/repos/" + Repo + "/releases/latest"

// DefaultMirrors are tried, in order, when GitHub itself is unreachable. A
// mirror is a URL prefix: "<mirror>/<original https URL>" (GitHub-proxy style).
// Users change the list with the "update_mirrors" setting; see README.
var DefaultMirrors = []string{"https://ghproxy.net/"}

const (
	// DefaultTimeout bounds each source, so a dead mirror costs seconds, not minutes.
	DefaultTimeout = 8 * time.Second
	maxAPIBytes    = 1 << 20 // release JSON
	maxSumsBytes   = 1 << 20 // SHA256SUMS
	maxMirrors     = 5
)

var (
	// ErrUnreachable means GitHub and every mirror failed.
	ErrUnreachable = errors.New("update source unreachable")
	ErrNoAsset     = errors.New("no download for this system")
	ErrNoChecksum  = errors.New("no checksum published for this file")
)

// Checker reads release data from GitHub, then from each mirror in turn.
type Checker struct {
	Client  *http.Client
	APIURL  string        // defaults to APIURL
	Mirrors []string      // URL prefixes; see DefaultMirrors
	Timeout time.Duration // per source; defaults to DefaultTimeout
}

// NewClient builds an HTTP client with short timeouts that goes through Rum's
// proxy setting (empty proxy = direct, honouring the usual environment).
func NewClient(proxy string) *http.Client {
	tr := http.DefaultTransport.(*http.Transport).Clone()
	tr.TLSHandshakeTimeout = DefaultTimeout
	tr.ResponseHeaderTimeout = DefaultTimeout
	if norm, ok := config.NormalizeProxy(proxy); ok && norm != "" {
		if u, err := url.Parse(norm); err == nil {
			tr.Proxy = http.ProxyURL(u)
		}
	}
	return &http.Client{Transport: tr}
}

// CleanMirrors keeps only usable https/http prefixes, trimmed and de-duplicated,
// capped at maxMirrors.
func CleanMirrors(in []string) []string {
	var out []string
	seen := map[string]bool{}
	for _, m := range in {
		m = strings.TrimSpace(m)
		u, err := url.Parse(m)
		if err != nil || u.Host == "" || (u.Scheme != "https" && u.Scheme != "http") {
			continue
		}
		m = strings.TrimRight(m, "/")
		if seen[m] || len(out) == maxMirrors {
			continue
		}
		seen[m] = true
		out = append(out, m)
	}
	return out
}

// sourceURLs lists the primary URL first, then one proxied URL per mirror.
func sourceURLs(primary string, mirrors []string) []string {
	out := []string{primary}
	for _, m := range CleanMirrors(mirrors) {
		out = append(out, m+"/"+primary)
	}
	return out
}

func (c *Checker) client() *http.Client {
	if c.Client != nil {
		return c.Client
	}
	return NewClient("")
}

func (c *Checker) timeout() time.Duration {
	if c.Timeout > 0 {
		return c.Timeout
	}
	return DefaultTimeout
}

// get fetches one URL within the per-source timeout and a size cap.
func (c *Checker) get(ctx context.Context, rawURL string, limit int64) ([]byte, error) {
	ctx, cancel := context.WithTimeout(ctx, c.timeout())
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("User-Agent", "Rum-update-check")
	resp, err := c.client().Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("status %d", resp.StatusCode)
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, limit+1))
	if err != nil {
		return nil, err
	}
	if int64(len(body)) > limit {
		return nil, errors.New("response too large")
	}
	return body, nil
}

// Latest returns the newest final release, trying GitHub then each mirror. A
// source that errors, answers non-200 or returns something that is not a release
// (a captive-portal page, say) is skipped.
func (c *Checker) Latest(ctx context.Context) (Release, error) {
	api := c.APIURL
	if api == "" {
		api = APIURL
	}
	var last error
	for _, src := range sourceURLs(api, c.Mirrors) {
		body, err := c.get(ctx, src, maxAPIBytes)
		if err == nil {
			var rel Release
			if rel, err = ParseRelease(body); err == nil {
				return rel, nil
			}
		}
		last = err
		if ctx.Err() != nil {
			break
		}
	}
	return Release{}, fmt.Errorf("%w: %v", ErrUnreachable, last)
}
