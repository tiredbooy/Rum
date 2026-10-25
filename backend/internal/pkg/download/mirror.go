package download

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net/url"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/tiredbooy/Rum/backend/internal/pkg/utils"
)

const (
	// MaxMirrors bounds the extra URLs a job may carry.
	MaxMirrors = 5
	// mirrorSampleLen is how much of the already-downloaded data (head and tail)
	// is compared byte for byte with the mirror before it is trusted.
	mirrorSampleLen = 64 << 10
	// mirrorVerifyTimeout bounds checking one candidate mirror.
	mirrorVerifyTimeout = 30 * time.Second
)

var errMirrorMismatch = errors.New("mirror is not the same file")

func mirrorValidationError(format string, a ...any) error {
	return &CodedError{Code: codeValidation, Kind: KindValidation, Err: fmt.Errorf(format, a...)}
}

// NormalizeMirrors trims, validates (http/https only) and de-duplicates a job's
// mirror list, dropping blanks and the primary URL itself. More than MaxMirrors
// distinct mirrors is an error.
func NormalizeMirrors(primary string, in []string) ([]string, error) {
	seen := map[string]bool{strings.TrimSpace(primary): true}
	var out []string
	for _, raw := range in {
		raw = strings.TrimSpace(raw)
		if raw == "" || seen[raw] {
			continue
		}
		if strings.ContainsAny(raw, "\r\n\x00") {
			return nil, mirrorValidationError("invalid mirror: contains a control character")
		}
		if err := ValidateURL(raw); err != nil {
			return nil, mirrorValidationError("invalid mirror %s: must be an http(s) link", RedactURL(raw))
		}
		seen[raw] = true
		out = append(out, raw)
	}
	if len(out) > MaxMirrors {
		return nil, mirrorValidationError("too many mirrors: at most %d", MaxMirrors)
	}
	return out, nil
}

func (j *Job) GetMirrors() []string {
	j.Mu.RLock()
	defer j.Mu.RUnlock()
	return append([]string(nil), j.Mirrors...)
}
func (j *Job) SetMirrors(v []string) { j.Mu.Lock(); defer j.Mu.Unlock(); j.Mirrors = v }
func (j *Job) GetActiveMirror() string {
	j.Mu.RLock()
	defer j.Mu.RUnlock()
	return j.ActiveMirror
}
func (j *Job) SetActiveMirror(v string) { j.Mu.Lock(); defer j.Mu.Unlock(); j.ActiveMirror = v }

// SetJobMirrors replaces a job's mirror list (validated and persisted) and
// returns the stored list. A running job picks the change up on its next run.
func (m *JobManager) SetJobMirrors(id string, mirrors []string) ([]string, error) {
	m.mu.RLock()
	job, ok := m.jobs[id]
	m.mu.RUnlock()
	if !ok {
		return nil, fmt.Errorf("job %s not found: %w", id, ErrNotFound)
	}
	list, err := NormalizeMirrors(job.GetURL(), mirrors)
	if err != nil {
		return nil, err
	}
	job.SetMirrors(list)
	m.saveToDisk()
	return list, nil
}

// region is a run of bytes [off, off+n) already written to the output file.
type region struct{ off, n int64 }

// mirrorSamples picks the head of the first and the tail of the last region.
func mirrorSamples(rs []region) []region {
	var have []region
	for _, r := range rs {
		if r.n > 0 {
			have = append(have, r)
		}
	}
	if len(have) == 0 {
		return nil
	}
	first, last := have[0], have[len(have)-1]
	a := region{first.off, min(first.n, mirrorSampleLen)}
	ln := min(last.n, mirrorSampleLen)
	b := region{last.off + last.n - ln, ln}
	if a == b {
		return []region{a}
	}
	return []region{a, b}
}

// fileRegions reports the contiguous prefix a single-stream download holds.
func fileRegions(path string) func() []region {
	return func() []region {
		if st, err := os.Stat(path); err == nil && st.Size() > 0 {
			return []region{{0, st.Size()}}
		}
		return nil
	}
}

// mirrorSet tracks which URL of a job is in use during one run and switches to
// a verified mirror when the current one fails.
type mirrorSet struct {
	mu       sync.Mutex
	job      *Job
	urls     []string // urls[0] is the primary
	cur      int
	dead     map[string]bool
	path     string
	regions  func() []region
	validate func() (etag, lastMod string)
}

func newMirrorSet(job *Job, primary, path string, regions func() []region, validate func() (string, string)) *mirrorSet {
	ms := &mirrorSet{job: job, urls: []string{primary}, dead: map[string]bool{}, path: path, regions: regions, validate: validate}
	for _, u := range job.GetMirrors() {
		if u != primary {
			ms.urls = append(ms.urls, u)
		}
	}
	job.SetActiveMirror("")
	return ms
}

func (ms *mirrorSet) current() string {
	ms.mu.Lock()
	defer ms.mu.Unlock()
	return ms.urls[ms.cur]
}

func (ms *mirrorSet) isPrimary(u string) bool { return u == ms.urls[0] }

// optFor returns opt with a Downloader safe for u: another host never receives
// the job's cookie or referer.
func (ms *mirrorSet) optFor(opt Options, u string) Options {
	if ms.isPrimary(u) || opt.Downloader == nil || sameAuthority(u, ms.urls[0]) {
		return opt
	}
	d := *opt.Downloader
	d.Headers = make(map[string]string, len(opt.Downloader.Headers))
	for k, v := range opt.Downloader.Headers {
		if k != "Cookie" && k != "Referer" && k != "Authorization" {
			d.Headers[k] = v
		}
	}
	opt.Downloader = &d
	return opt
}

// sameAuthority compares host:port, so a mirror on another port or host counts
// as a different party for credentials.
func sameAuthority(a, b string) bool {
	ua, err1 := url.Parse(a)
	ub, err2 := url.Parse(b)
	return err1 == nil && err2 == nil && strings.EqualFold(ua.Host, ub.Host)
}

// run calls fn with the current URL; when it fails with an error a mirror can
// fix, it moves to the next verified mirror and calls fn again. fn must resume
// from what is already on disk.
func (ms *mirrorSet) run(ctx context.Context, opt Options, fn func(ctx context.Context, u string, o Options) error) error {
	for {
		cur := ms.current()
		err := fn(ctx, cur, ms.optFor(opt, cur))
		if err == nil || !ms.failover(ctx, opt, cur, err) {
			return err
		}
	}
}

// mirrorFailoverEligible: the failure is about the link or the route to it, so
// another URL for the same file may succeed.
func mirrorFailoverEligible(err error) bool {
	if err == nil || errors.Is(err, context.Canceled) {
		return false
	}
	code, _ := ClassifyJobError(err)
	switch code {
	case ErrCodeBlockedPage, ErrCodeLinkExpired, ErrorCodeStalled, ErrCodeSiteUnreachable,
		ErrCodeDNS, ErrCodeNetworkDown, ErrCodeTimeout, ErrCodeServerError, ErrCodeProxy, ErrCodeTLS:
		return true
	}
	return false
}

// failover switches away from the failed URL to the first mirror that passes
// verification. It reports whether fn should be run again. Concurrent segment
// workers share one decision: whoever fails first verifies, the rest just
// follow the new current URL.
func (ms *mirrorSet) failover(ctx context.Context, opt Options, failed string, cause error) bool {
	if len(ms.urls) < 2 || ctx.Err() != nil || !mirrorFailoverEligible(cause) {
		return false
	}
	ms.mu.Lock()
	defer ms.mu.Unlock()
	if ms.urls[ms.cur] != failed {
		return !ms.dead[ms.urls[ms.cur]]
	}
	ms.dead[failed] = true
	for i, cand := range ms.urls {
		if ms.dead[cand] {
			continue
		}
		if err := ms.verify(ctx, ms.optFor(opt, cand), cand); err != nil {
			DebugLog("mirror rejected " + RedactURL(cand) + ": " + redactErr(err))
			ms.dead[cand] = true
			continue
		}
		ms.cur = i
		host := ""
		if i != 0 {
			if u, err := url.Parse(cand); err == nil {
				host = u.Host
			}
		}
		ms.job.SetActiveMirror(host)
		DebugLog("failover to mirror " + RedactURL(cand))
		return true
	}
	return false
}

// verify proves cand serves the same file as the bytes already on disk: same
// size, validators that do not conflict (ValidateResume) and, when data exists,
// identical head and tail samples fetched with Range. Different servers rarely
// share ETags, so matching samples decide; validators only decide when there is
// nothing downloaded yet to compare.
func (ms *mirrorSet) verify(ctx context.Context, opt Options, cand string) error {
	ctx, cancel := context.WithTimeout(ctx, mirrorVerifyTimeout)
	defer cancel()
	d := opt.Downloader
	info, err := d.HeadWithFallbackContext(ctx, cand)
	if err != nil {
		return fmt.Errorf("mirror not reachable: %w", err)
	}
	if info.StatusCode < 200 || info.StatusCode >= 400 {
		return fmt.Errorf("mirror answered HTTP %d: %w", info.StatusCode, errMirrorMismatch)
	}
	total := ms.job.GetTotalSize()
	size := utils.ConvertSizeToInt(info.ContentSize)
	if total <= 0 || size != total {
		return fmt.Errorf("size %d, want %d: %w", size, total, errMirrorMismatch)
	}
	var sEtag, sLM string
	if ms.validate != nil {
		sEtag, sLM = ms.validate()
	}
	samples := mirrorSamples(ms.regions())
	if len(samples) == 0 {
		if !info.SupportsRange || !ValidateResume(info.ETag, info.LastModified, sEtag, sLM, size, total) {
			return fmt.Errorf("no range support or conflicting validators: %w", errMirrorMismatch)
		}
		return nil
	}
	f, err := os.Open(ms.path)
	if err != nil {
		return err
	}
	defer f.Close()
	for _, s := range samples {
		local := make([]byte, s.n)
		if n, err := f.ReadAt(local, s.off); n != len(local) {
			return fmt.Errorf("read local sample: %v: %w", err, errMirrorMismatch)
		}
		remote, err := fetchSample(ctx, d, cand, s)
		if err != nil {
			return fmt.Errorf("fetch sample: %v: %w", err, errMirrorMismatch)
		}
		if !bytes.Equal(local, remote) {
			return fmt.Errorf("bytes %d+%d differ: %w", s.off, s.n, errMirrorMismatch)
		}
	}
	return nil
}

// fetchSample reads exactly the bytes of r from u via a Range request; anything
// but a matching 206 is an error.
func fetchSample(ctx context.Context, d *Downloader, u string, r region) ([]byte, error) {
	req, err := d.NewRequest("GET", u)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Range", fmt.Sprintf("bytes=%d-%d", r.off, r.off+r.n-1))
	resp, err := d.Client.Do(req.WithContext(ctx))
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != 206 {
		return nil, fmt.Errorf("HTTP %d, want 206", resp.StatusCode)
	}
	if cr := resp.Header.Get("Content-Range"); !strings.HasPrefix(cr, "bytes "+strconv.FormatInt(r.off, 10)+"-") {
		return nil, fmt.Errorf("unexpected Content-Range %q", cr)
	}
	buf := make([]byte, r.n)
	if _, err := io.ReadFull(resp.Body, buf); err != nil {
		return nil, err
	}
	return buf, nil
}
