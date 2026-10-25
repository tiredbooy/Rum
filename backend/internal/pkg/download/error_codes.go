package download

import (
	"errors"
	"net/http"
	"net/url"
)

// Machine-readable Job.ErrorCode values. The free-text Job.Error stays; the code
// lets the UI pick a tailored action without parsing messages.
const (
	// ErrCodeLinkExpired marks a failure where the server answered 403/404/410 for
	// a download that was working: the (often signed or time-limited) link no
	// longer serves the file. The UI offers "Paste new link".
	ErrCodeLinkExpired = "link_expired"
)

// linkExpiredCode returns ErrCodeLinkExpired when err is (or wraps) an HTTP
// 403/404/410 response, and "" otherwise.
func linkExpiredCode(err error) string {
	var se *httpStatusError
	if errors.As(err, &se) {
		switch se.code {
		case http.StatusForbidden, http.StatusNotFound, http.StatusGone:
			return ErrCodeLinkExpired
		}
	}
	return ""
}

// RedactURL drops everything from a URL that can carry credentials or tokens
// (userinfo, query, fragment) so it is safe to write to logs.
func RedactURL(raw string) string {
	u, err := url.Parse(raw)
	if err != nil || u.Host == "" {
		return "<redacted>"
	}
	return u.Scheme + "://" + u.Host + u.Path
}
