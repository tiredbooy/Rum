package ytdlp

import (
	"bufio"
	"context"
	"errors"
	"io"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"
)

const (
	progressPrefix = "RUMP|"
	filePrefix     = "RUMFILE|"
	// maxBaseRunes caps the title-derived file name so the path stays valid.
	maxBaseRunes = 120
)

// Spec describes one download run.
type Spec struct {
	Bin       string
	URL       string
	Dir       string // destination folder
	Base      string // file name without extension (already sanitized)
	Selector  string // yt-dlp -f expression
	Proxy     string
	LimitKBps int // 0 = unlimited
}

// SafeBase turns a title into a file name stem: no separators or reserved
// characters, no '%' (yt-dlp would read it as a template field), bounded length.
func SafeBase(title string) string {
	var b strings.Builder
	for _, r := range title {
		switch {
		case r < 0x20 || r == 0x7f:
		case strings.ContainsRune(`/\:*?"<>|%`, r):
			b.WriteRune('_')
		default:
			b.WriteRune(r)
		}
	}
	s := strings.Trim(strings.TrimSpace(b.String()), ". ")
	if r := []rune(s); len(r) > maxBaseRunes {
		s = strings.TrimSpace(string(r[:maxBaseRunes]))
	}
	if s == "" {
		return "video"
	}
	return s
}

// BuildArgs is the full, shell-free argument list for a download. No config
// files (--ignore-config), no --exec, and the URL comes after `--`.
func BuildArgs(s Spec) []string {
	args := []string{
		"--ignore-config", "--no-playlist", "--continue", "--newline", "--no-colors", "--no-mtime",
		"--progress",
		"--progress-template", "download:" + progressPrefix +
			"%(progress.status)s|%(progress.downloaded_bytes)s|%(progress.total_bytes)s|" +
			"%(progress.total_bytes_estimate)s|%(progress.speed)s|%(progress.eta)s",
		"--print", "after_move:" + filePrefix + "%(filepath)s",
		"-f", s.Selector,
		"-o", filepath.Join(s.Dir, s.Base) + ".%(ext)s",
	}
	if s.Proxy != "" {
		args = append(args, "--proxy", s.Proxy)
	}
	if s.LimitKBps > 0 {
		args = append(args, "--limit-rate", strconv.Itoa(s.LimitKBps)+"K")
	}
	return append(args, "--", s.URL)
}

// Progress is one parsed progress report.
type Progress struct {
	Status     string // downloading | finished | ...
	Downloaded int64
	Total      int64
	Speed      float64 // bytes/s
	ETA        time.Duration
	Percent    float64 // -1 when only bytes are known
}

func numOr0(s string) float64 {
	f, err := strconv.ParseFloat(strings.TrimSpace(s), 64)
	if err != nil || f < 0 {
		return 0
	}
	return f
}

var stdLine = regexp.MustCompile(`^\[download\]\s+(\d+(?:\.\d+)?)%(?:\s+of\s+~?\s*[\d.]+\S+)?(?:\s+at\s+(\S+))?(?:\s+ETA\s+(\S+))?`)

// ParseProgressLine reads either Rum's machine template line or yt-dlp's stock
// "[download]  45.3% of ... ETA 00:05" line. ok is false for anything else.
func ParseProgressLine(line string) (Progress, bool) {
	line = strings.TrimSpace(line)
	if rest, found := strings.CutPrefix(line, progressPrefix); found {
		f := strings.Split(rest, "|")
		if len(f) < 6 {
			return Progress{}, false
		}
		total := numOr0(f[2])
		if total == 0 {
			total = numOr0(f[3])
		}
		return Progress{
			Status:     strings.TrimSpace(f[0]),
			Downloaded: int64(numOr0(f[1])),
			Total:      int64(total),
			Speed:      numOr0(f[4]),
			ETA:        time.Duration(numOr0(f[5])) * time.Second,
			Percent:    -1,
		}, true
	}
	m := stdLine.FindStringSubmatch(line)
	if m == nil {
		return Progress{}, false
	}
	pct, _ := strconv.ParseFloat(m[1], 64)
	p := Progress{Status: "downloading", Percent: pct, ETA: parseClock(m[3])}
	if pct >= 100 {
		p.Status = "finished"
	}
	return p, true
}

func parseClock(s string) time.Duration {
	if s == "" {
		return 0
	}
	var secs int
	for _, part := range strings.Split(s, ":") {
		n, err := strconv.Atoi(part)
		if err != nil {
			return 0
		}
		secs = secs*60 + n
	}
	return time.Duration(secs) * time.Second
}

// Tracker folds yt-dlp's per-stream reports into one monotonic job progress.
// A merged download fetches video then audio, each counting from zero; the
// bytes of finished streams are carried over so the bar does not jump back.
type Tracker struct {
	Estimate int64 // expected total for the whole job, 0 if unknown
	base     int64
	last     int64
	lastTot  int64
}

// Feed returns the job-level (downloaded, total) after a report.
func (t *Tracker) Feed(p Progress) (downloaded, total int64) {
	if p.Percent >= 0 { // stock line: only a percentage is known
		if t.Estimate > 0 {
			return int64(p.Percent / 100 * float64(t.Estimate)), t.Estimate
		}
		return int64(p.Percent * 10), 1000
	}
	cur, curTotal := p.Downloaded, p.Total
	if cur < t.last && p.Status != "finished" { // a new stream began without a "finished"
		t.base += t.lastTot
	}
	t.last, t.lastTot = cur, curTotal
	downloaded = t.base + cur
	total = t.base + curTotal
	if p.Status == "finished" {
		if curTotal < cur {
			curTotal = cur
		}
		t.base += curTotal
		t.last, t.lastTot = 0, 0
		downloaded, total = t.base, t.base
	}
	if t.Estimate > total {
		total = t.Estimate
	}
	if downloaded > total {
		total = downloaded
	}
	return downloaded, total
}

// tailBuffer keeps the last max bytes written.
type tailBuffer struct {
	mu  sync.Mutex
	max int
	buf []byte
}

func (t *tailBuffer) Write(p []byte) (int, error) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.buf = append(t.buf, p...)
	if len(t.buf) > t.max {
		t.buf = t.buf[len(t.buf)-t.max:]
	}
	return len(p), nil
}

func (t *tailBuffer) String() string {
	t.mu.Lock()
	defer t.mu.Unlock()
	return string(t.buf)
}

// limitWriter errors once more than n bytes are written, bounding memory.
type limitWriter struct {
	w io.Writer
	n int
}

func (l *limitWriter) Write(p []byte) (int, error) {
	if len(p) > l.n {
		return 0, errors.New("output too large")
	}
	l.n -= len(p)
	return l.w.Write(p)
}

func isNotFound(err error) bool {
	return errors.Is(err, exec.ErrNotFound) || errors.Is(err, fs.ErrNotExist)
}

// Run executes yt-dlp for s, reporting progress as it goes, and returns the
// finished file's path. Cancelling ctx kills the process tree (pause/cancel);
// ctx's error is then returned and the .part files stay so a later run resumes.
func Run(ctx context.Context, s Spec, onProgress func(Progress)) (string, error) {
	if err := ValidateURL(s.URL); err != nil {
		return "", &Error{Code: CodeVideoUnavailable, Message: "Link must start with http or https"}
	}
	if err := os.MkdirAll(s.Dir, 0o755); err != nil {
		return "", &Error{Code: "permission", Message: "Can't write to folder"}
	}
	cmd := exec.CommandContext(ctx, s.Bin, BuildArgs(s)...)
	setProcAttr(cmd)
	cmd.Cancel = func() error { return killTree(cmd) }
	cmd.WaitDelay = 5 * time.Second
	stderr := &tailBuffer{max: 8192}
	cmd.Stderr = stderr
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return "", &Error{Code: CodeFailed, Message: "Couldn't start yt-dlp"}
	}
	if err := cmd.Start(); err != nil {
		if isNotFound(err) {
			return "", missingError()
		}
		return "", &Error{Code: CodeFailed, Message: "Couldn't start yt-dlp"}
	}

	var out string
	sc := bufio.NewScanner(stdout)
	sc.Buffer(make([]byte, 64<<10), 1<<20)
	for sc.Scan() {
		line := sc.Text()
		if p, ok := strings.CutPrefix(strings.TrimSpace(line), filePrefix); ok {
			out = strings.TrimSpace(p)
			continue
		}
		if p, ok := ParseProgressLine(line); ok && onProgress != nil {
			onProgress(p)
		}
	}
	werr := cmd.Wait()
	if ctx.Err() != nil {
		return "", ctx.Err()
	}
	if werr != nil {
		return "", ClassifyStderr(stderr.String())
	}
	if out == "" {
		out = findFinished(s.Dir, s.Base)
	}
	if out == "" {
		return "", &Error{Code: CodeFailed, Message: "yt-dlp finished without a file"}
	}
	return out, nil
}

// findFinished locates "<base>.<ext>" when yt-dlp did not report the path.
func findFinished(dir, base string) string {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return ""
	}
	for _, e := range entries {
		n := e.Name()
		if e.IsDir() || !strings.HasPrefix(n, base+".") || intermediate.MatchString(strings.TrimPrefix(n, base)) {
			continue
		}
		return filepath.Join(dir, n)
	}
	return ""
}
