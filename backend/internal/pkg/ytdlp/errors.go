// Package ytdlp drives the yt-dlp command line tool: locating or installing the
// binary, listing a page's formats, and running a download as a managed
// subprocess. It never uses a shell and never imports the download engine.
package ytdlp

import (
	"errors"
	"strings"
)

// Machine error codes. They are part of the API contract (Job.ErrorCode).
const (
	CodeVideoUnavailable = "video_unavailable"
	CodeMissing          = "ytdlp_missing"
	CodeFailed           = "ytdlp_failed"
)

// Error is a yt-dlp failure with a user-facing code. Code is either one of the
// Code* values above or an engine code reused for network-level failures
// ("dns", "timeout", "network_down", "proxy", "tls", "disk_full", "permission").
type Error struct {
	Code      string
	Message   string // short, safe to show
	Retryable bool
}

func (e *Error) Error() string { return e.Message }

// AsError returns err's *Error, if any.
func AsError(err error) (*Error, bool) {
	var e *Error
	if errors.As(err, &e) {
		return e, true
	}
	return nil, false
}

func missingError() *Error {
	return &Error{Code: CodeMissing, Message: "yt-dlp is not installed"}
}

// stderrRules map a lowercase fragment of yt-dlp's stderr to an error. Order
// matters: the first match wins, so specific causes precede generic ones.
var stderrRules = []struct {
	frag  string
	code  string
	msg   string
	retry bool
}{
	{"no space left", "disk_full", "Disk is full", false},
	{"permission denied", "permission", "Can't write to folder", false},
	{"video unavailable", CodeVideoUnavailable, "Video unavailable", false},
	{"private video", CodeVideoUnavailable, "Video is private", false},
	{"this video is not available", CodeVideoUnavailable, "Video unavailable", false},
	{"has been removed", CodeVideoUnavailable, "Video was removed", false},
	{"not available in your country", CodeVideoUnavailable, "Video blocked in your region", false},
	{"geo restriction", CodeVideoUnavailable, "Video blocked in your region", false},
	{"sign in to confirm", CodeVideoUnavailable, "Site asks for sign-in", false},
	{"login required", CodeVideoUnavailable, "Site asks for sign-in", false},
	{"unsupported url", CodeVideoUnavailable, "No video found at this link", false},
	{"is unavailable", CodeVideoUnavailable, "Video unavailable", false},
	{"proxy", "proxy", "Proxy failed", true},
	{"name resolution", "dns", "Site not found", true},
	{"getaddrinfo failed", "dns", "Site not found", true},
	{"no such host", "dns", "Site not found", true},
	{"timed out", "timeout", "Server took too long", true},
	{"certificate", "tls", "Secure connection failed", false},
	{"connection reset", "network_down", "Connection lost", true},
	{"connection refused", "network_down", "Connection lost", true},
	{"network is unreachable", "network_down", "Connection lost", true},
	{"unable to download", "network_down", "Connection lost", true},
}

// ClassifyStderr turns yt-dlp's stderr into a coded error. Unknown output is
// ytdlp_failed; the message never carries raw output (it can hold URLs/tokens).
func ClassifyStderr(stderr string) *Error {
	low := strings.ToLower(stderr)
	for _, r := range stderrRules {
		if strings.Contains(low, r.frag) {
			return &Error{Code: r.code, Message: r.msg, Retryable: r.retry}
		}
	}
	return &Error{Code: CodeFailed, Message: "yt-dlp failed", Retryable: false}
}
