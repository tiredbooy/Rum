package download

import (
	"bufio"
	"context"
	"errors"
	"io"
	"mime"
	"net/http"
	"path/filepath"
	"strings"
)

// sniffLen is how many leading bytes are inspected when the server gave no
// useful Content-Type.
const sniffLen = 512

// blockedPageError reports that the server returned a web page where a
// non-HTML file was expected: typically a captive portal, an ISP block page or a
// login wall. It is permanent (never auto-retried): retrying only fetches the
// same page again, and writing it would corrupt the download.
type blockedPageError struct{}

func (*blockedPageError) Error() string {
	return "Server returned a web page (blocked or login needed)"
}

// jobBlocked reports whether the job failed because of a block page, either from
// the live error or the persisted code (the typed error is lost across restarts).
func jobBlocked(j *Job) bool {
	if code, _ := j.GetErrorCode(); code == ErrCodeBlockedPage {
		return true
	}
	var bp *blockedPageError
	return errors.As(j.GetError(), &bp)
}

type blockJobKey struct{}

// withBlockJob lets segment workers (which have no job parameter) run the same
// block-page check as the single-stream path.
func withBlockJob(ctx context.Context, job *Job) context.Context {
	return context.WithValue(ctx, blockJobKey{}, job)
}

func blockJobFrom(ctx context.Context) *Job {
	j, _ := ctx.Value(blockJobKey{}).(*Job)
	return j
}

var htmlExts = map[string]bool{".html": true, ".htm": true, ".xhtml": true, ".shtml": true}

// expectsHTML reports whether the job legitimately downloads a web page. The
// file extension wins; the originally recorded Content-Type decides only for
// names without one.
func expectsHTML(job *Job) bool {
	if ext := strings.ToLower(filepath.Ext(job.GetFileName())); ext != "" {
		return htmlExts[ext]
	}
	return isHTMLType(job.ContentType)
}

func isHTMLType(ct string) bool {
	mt, _, err := mime.ParseMediaType(ct)
	if err != nil {
		mt = strings.ToLower(strings.TrimSpace(strings.SplitN(ct, ";", 2)[0]))
	}
	return mt == "text/html" || mt == "application/xhtml+xml"
}

// isGenericType reports a missing or catch-all Content-Type, where the body must
// be sniffed to know what it really is.
func isGenericType(ct string) bool {
	if strings.TrimSpace(ct) == "" {
		return true
	}
	mt, _, err := mime.ParseMediaType(ct)
	if err != nil {
		return false
	}
	switch mt {
	case "application/octet-stream", "binary/octet-stream", "application/x-download",
		"application/force-download", "application/download":
		return true
	}
	return false
}

// checkBlockedPage inspects a download response before any byte is written. It
// returns *blockedPageError for an HTML response to a non-HTML job (declared, or
// sniffed from the first 512 bytes of a 200 body whose type is missing/generic).
// Sniffing buffers the body, so resp.Body is replaced with an equivalent reader.
// The one-time override (Job.AllowBlockedPage) disables the check.
func checkBlockedPage(job *Job, resp *http.Response) error {
	if job == nil || job.GetAllowBlockedPage() || expectsHTML(job) {
		return nil
	}
	ct := resp.Header.Get("Content-Type")
	if isHTMLType(ct) {
		return &blockedPageError{}
	}
	if resp.StatusCode != http.StatusOK || !isGenericType(ct) {
		return nil
	}
	br := bufio.NewReaderSize(resp.Body, 4096)
	head, _ := br.Peek(sniffLen) // short body / EOF is fine: sniff what we have
	resp.Body = struct {
		io.Reader
		io.Closer
	}{br, resp.Body}
	if strings.HasPrefix(http.DetectContentType(head), "text/html") {
		return &blockedPageError{}
	}
	return nil
}
