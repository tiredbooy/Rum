package dto

import (
	"net/url"
	"strings"
)

// Limits for the request headers a browser extension may forward.
const (
	maxExtCookieLen   = 16 << 10
	maxExtRefererLen  = 4 << 10
	maxExtUserAgent   = 1 << 10
	maxExtFilenameLen = 512
)

// ExtAddRequest is the body of POST /ext/add: one link plus the request context
// the browser had when it started the download. Cookie is a ready-made Cookie
// header value and is a secret: it is stored for resume only, never echoed back
// or logged.
type ExtAddRequest struct {
	URL       string `json:"url"`
	Filename  string `json:"filename,omitempty"`
	Referer   string `json:"referer,omitempty"`
	UserAgent string `json:"user_agent,omitempty"`
	Cookie    string `json:"cookie,omitempty"`
	// AutoStart defaults to true when omitted.
	AutoStart *bool `json:"auto_start,omitempty"`
}

// Validate returns field -> problem, or nil.
func (r *ExtAddRequest) Validate() map[string]string {
	f := map[string]string{}
	raw := strings.TrimSpace(r.URL)
	if u, err := url.Parse(raw); raw == "" || err != nil || u.Host == "" || (u.Scheme != "http" && u.Scheme != "https") {
		f["url"] = "must be an absolute http(s) URL"
	}
	for name, v := range map[string]struct {
		val string
		max int
	}{
		"cookie":     {r.Cookie, maxExtCookieLen},
		"referer":    {r.Referer, maxExtRefererLen},
		"user_agent": {r.UserAgent, maxExtUserAgent},
		"filename":   {r.Filename, maxExtFilenameLen},
	} {
		if len(v.val) > v.max {
			f[name] = "too long"
		} else if strings.ContainsAny(v.val, "\r\n\x00") {
			f[name] = "contains an invalid character"
		}
	}
	if len(f) == 0 {
		return nil
	}
	return f
}

// StartNow reports whether the job should start immediately.
func (r *ExtAddRequest) StartNow() bool { return r.AutoStart == nil || *r.AutoStart }
