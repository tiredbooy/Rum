package download

import (
	"bufio"
	"context"
	"errors"
	"io"
	"mime"
	"net/http"
	"path/filepath"
	"strconv"
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

// textExts are extensions whose files are legitimately plain text, so a
// text/plain answer for them is never suspicious.
var textExts = map[string]bool{
	".txt": true, ".log": true, ".md": true, ".csv": true, ".tsv": true, ".json": true,
	".xml": true, ".yaml": true, ".yml": true, ".ini": true, ".conf": true, ".cfg": true,
	".toml": true, ".srt": true, ".vtt": true, ".sub": true, ".ass": true, ".ssa": true,
	".css": true, ".js": true, ".ts": true, ".py": true, ".go": true, ".c": true, ".h": true,
	".cpp": true, ".java": true, ".sh": true, ".bat": true, ".sql": true, ".tex": true,
	".rtf": true, ".svg": true, ".m3u": true, ".m3u8": true, ".nfo": true, ".php": true,
	".rb": true, ".rs": true, ".pl": true, ".lua": true, ".diff": true, ".patch": true,
}

// expectsBinary reports whether the job is clearly a binary file: by extension
// (anything not known to be text) or, for names without one, by the originally
// recorded Content-Type. Unknown stays false so text is never wrongly rejected.
func expectsBinary(job *Job) bool {
	if ext := strings.ToLower(filepath.Ext(job.GetFileName())); ext != "" {
		return !textExts[ext] && !htmlExts[ext]
	}
	ct := strings.ToLower(strings.TrimSpace(job.ContentType))
	if ct == "" || isGenericType(ct) {
		return false
	}
	for _, p := range []string{"text/", "application/json", "application/xml", "application/javascript"} {
		if strings.HasPrefix(ct, p) {
			return false
		}
	}
	return true
}

func isPlainTextType(ct string) bool {
	mt, _, err := mime.ParseMediaType(ct)
	if err != nil {
		mt = strings.ToLower(strings.TrimSpace(strings.SplitN(ct, ";", 2)[0]))
	}
	return mt == "text/plain"
}

// startsAtFileHead reports whether the body begins at byte 0 of the file, so a
// text sniff is meaningful (a middle segment of a binary file may look like text).
func startsAtFileHead(resp *http.Response) bool {
	if resp.StatusCode != http.StatusPartialContent {
		return true
	}
	cr := strings.TrimSpace(resp.Header.Get("Content-Range"))
	return strings.HasPrefix(cr, "bytes 0-")
}

// checkBlockedPage inspects a download response before any byte is written. It
// returns *blockedPageError for an HTML response to a non-HTML job (declared, or
// sniffed from the first 512 bytes of a 200/206 body whose type is missing or
// generic). A text/plain answer to a clearly binary job is also suspicious when
// the body sniffs as text. Sniffing buffers the body, so resp.Body is replaced
// with an equivalent reader. The one-time override (Job.AllowBlockedPage)
// disables the check.
func checkBlockedPage(job *Job, resp *http.Response) error {
	if job == nil || job.GetAllowBlockedPage() || expectsHTML(job) {
		return nil
	}
	ct := resp.Header.Get("Content-Type")
	if isHTMLType(ct) {
		return &blockedPageError{}
	}
	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusPartialContent {
		return nil
	}
	plain := isPlainTextType(ct) && expectsBinary(job) && startsAtFileHead(resp)
	if !plain && !isGenericType(ct) {
		return nil
	}
	br := bufio.NewReaderSize(resp.Body, 4096)
	head, _ := br.Peek(sniffLen) // short body / EOF is fine: sniff what we have
	resp.Body = struct {
		io.Reader
		io.Closer
	}{br, resp.Body}
	sniffed := http.DetectContentType(head)
	if strings.HasPrefix(sniffed, "text/html") || (plain && len(head) > 0 && strings.HasPrefix(sniffed, "text/")) {
		return &blockedPageError{}
	}
	return nil
}

// probeBlockedPage validates the first response of a fresh segmented download
// (status, Content-Type, 512-byte sniff) BEFORE the output file is allocated, so
// a block page never leaves a pre-sized, zero-filled file behind. It fetches
// only the head of the file. Only a block page is reported: any other probe
// failure is ignored, because the real workers surface and retry those.
func probeBlockedPage(ctx context.Context, opt Options, job *Job, url string) error {
	if job == nil || job.GetAllowBlockedPage() || expectsHTML(job) {
		return nil
	}
	req, err := opt.Downloader.NewRequest("GET", url)
	if err != nil {
		return nil
	}
	req.Header.Set("Range", "bytes=0-"+strconv.Itoa(sniffLen-1))
	resp, err := opt.Downloader.Client.Do(req.WithContext(ctx))
	if err != nil {
		return nil
	}
	defer resp.Body.Close()
	return checkBlockedPage(job, resp)
}

func jobBlockedErr(err error) bool {
	var bp *blockedPageError
	return errors.As(err, &bp)
}
