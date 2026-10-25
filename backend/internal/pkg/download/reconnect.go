package download

import (
	"context"
	"log"
	"net"
	"net/http"
	"net/url"
	"sync"
	"time"

	"github.com/tiredbooy/Rum/backend/internal/pkg/config"
)

// reconnectTickInterval is how often the controller re-checks failed downloads.
// 20s is responsive enough to feel automatic after a Wi-Fi drop without turning
// a long outage into a connection-attempt storm.
const reconnectTickInterval = 20 * time.Second

// maxReconnectAttempts bounds how many CONSECUTIVE auto-resumes of a single job
// may end without any new bytes. A download that keeps failing the moment the
// host is reachable is not a connectivity problem, and retrying it forever would
// hide a real error from the user. Any progress since the last failure resets
// the count, so a slow link that keeps dropping but keeps advancing is never
// abandoned, while a zero-progress loop still stops (the global idle cap).
const maxReconnectAttempts = 10

// reconnectProbeTimeout bounds the per-host reachability probe.
const reconnectProbeTimeout = 4 * time.Second

// ResumeController implements the AutoResumeOnReconnect setting: it watches for
// downloads that failed with a transient NETWORK error and re-queues them once
// their host is reachable again.
//
// Before this existed, auto_resume_on_reconnect was persisted, exposed as a
// switch in the settings UI, and read by nothing at all — flipping it changed
// no behaviour anywhere in the codebase. The engine's retry/backoff only covers
// failures *within* one download attempt; once those retries are exhausted the
// job goes to "error" and stays there, which is exactly the case a user means
// by "continue when the network comes back".
//
// It runs a single context-cancelable ticker goroutine; Stop cancels it and
// waits for it to exit, so it is leak-free.
type ResumeController struct {
	manager  *JobManager
	interval time.Duration

	// probe reports whether the download's host is reachable the same way the
	// engine would reach it (through the configured proxy). target is the job
	// URL. Injectable so tests don't touch the network.
	probe func(ctx context.Context, target string) bool

	// attempts counts consecutive zero-progress auto-resumes per job id, so one
	// permanently-broken download cannot be retried forever. lastDone remembers
	// the job's byte count at its last auto-resume to detect progress. Only
	// touched from the ticker goroutine.
	attempts map[string]int
	lastDone map[string]int64

	// now is the clock used for the night-window check; injectable for tests.
	now func() time.Time

	cancel context.CancelFunc
	wg     sync.WaitGroup
	once   sync.Once
}

// NewResumeController builds a controller bound to the given manager. Settings
// are re-read on every tick so toggling the preference takes effect without a
// restart.
func NewResumeController(manager *JobManager) *ResumeController {
	return &ResumeController{
		manager:  manager,
		interval: reconnectTickInterval,
		probe:    dialProbe,
		attempts: make(map[string]int),
		lastDone: make(map[string]int64),
		now:      time.Now,
	}
}

// Start launches the ticker goroutine. Calling Start more than once is safe but
// only the first call has effect.
func (c *ResumeController) Start(ctx context.Context) {
	runCtx, cancel := context.WithCancel(ctx)
	c.cancel = cancel

	c.wg.Add(1)
	go func() {
		defer c.wg.Done()
		ticker := time.NewTicker(c.interval)
		defer ticker.Stop()
		for {
			select {
			case <-runCtx.Done():
				return
			case <-ticker.C:
				c.tick(runCtx)
			}
		}
	}()
}

// Stop cancels the ticker goroutine and waits for it to exit. It is idempotent.
func (c *ResumeController) Stop() {
	c.once.Do(func() {
		if c.cancel != nil {
			c.cancel()
		}
		c.wg.Wait()
	})
}

// tick re-queues every eligible failed job whose host has become reachable.
func (c *ResumeController) tick(ctx context.Context) {
	if c.manager == nil {
		return
	}

	var setting config.Setting
	if err := setting.LoadSettingMetadata(); err != nil {
		return
	}
	if !setting.AutoResumeOnReconnect {
		return
	}

	for _, job := range c.manager.GetAllJobs() {
		if ctx.Err() != nil {
			return
		}
		id := job.ID
		if job.GetStatus() == StatusCompleted {
			// Done: forget it so a re-download of the same id starts fresh.
			delete(c.attempts, id)
			delete(c.lastDone, id)
			continue
		}
		if job.GetStatus() != StatusError {
			continue
		}
		if !c.eligible(job) {
			continue // a 404 / checksum mismatch / full disk is not a connectivity problem
		}
		// Progress since the last auto-resume means the link works, just badly:
		// give the job a fresh budget.
		done := job.GetDownloaded()
		if prev, seen := c.lastDone[id]; seen && done > prev {
			c.attempts[id] = 0
		}
		if c.attempts[id] >= maxReconnectAttempts {
			continue
		}
		if _, ok := hostPortForURL(job.GetURL()); !ok || !c.probe(ctx, job.GetURL()) {
			continue // still offline (or an unusable URL) — try again next tick
		}

		c.attempts[id]++
		c.lastDone[id] = done
		// StartJob only accepts pending/paused, so move the failed job back to
		// pending first. The partial data and resume sidecar on disk make this a
		// real resume, not a restart from zero.
		job.SetStatus(StatusPending)
		job.SetError(nil)
		if err := c.manager.StartJob(ctx, id); err != nil {
			log.Printf("auto-resume on reconnect: %s: %v", id, err)
			job.SetStatus(StatusError)
		}
	}
}

// eligible reports whether a failed job may be auto-resumed on reconnect. Disk
// full and block pages are never eligible (they need the user or the
// DiskWatcher); otherwise the live typed error decides, falling back to the
// persisted classifier flag because a job reloaded after a restart only has the
// error text.
func (c *ResumeController) eligible(job *Job) bool {
	code, retryable := job.GetErrorCode()
	if c.manager != nil && c.manager.NightHolds(job, c.now()) {
		return false // a night job waits for its window; the night plan restarts it
	}
	if jobBlocked(job) || code == ErrCodeDiskFull || code == ErrCodeLinkExpired {
		return false
	}
	if _, ok := AsDiskFull(job.GetError()); ok {
		return false
	}
	return isRetryableError(job.GetError()) || (code != "" && retryable)
}

// hostPortForURL turns a download URL into a dialable "host:port", defaulting
// the port from the scheme.
func hostPortForURL(raw string) (string, bool) {
	u, err := url.Parse(raw)
	if err != nil || u.Host == "" {
		return "", false
	}
	if u.Port() != "" {
		return u.Host, true
	}
	switch u.Scheme {
	case "https":
		return net.JoinHostPort(u.Hostname(), "443"), true
	case "http":
		return net.JoinHostPort(u.Hostname(), "80"), true
	default:
		return "", false
	}
}

// dialProbe reports whether the download's host answers, going through the same
// proxy the engine uses. The proxy setting is re-read on every request (not
// cached), so changing it applies to the next probe without a restart. A direct
// TCP dial would report "offline" behind a mandatory proxy/VPN and report
// "online" for a host the proxy cannot reach. Any HTTP response, even an error
// status, proves the path works. Probing the download's OWN host (not some
// third-party endpoint) sends no traffic anywhere the user was not already
// downloading from.
func dialProbe(ctx context.Context, target string) bool {
	tr := &http.Transport{
		Proxy: func(req *http.Request) (*url.URL, error) {
			if fn := proxyTransportFunc(); fn != nil {
				return fn(req)
			}
			return nil, nil
		},
		DialContext:         (&net.Dialer{Timeout: reconnectProbeTimeout}).DialContext,
		TLSHandshakeTimeout: reconnectProbeTimeout,
		DisableKeepAlives:   true,
	}
	defer tr.CloseIdleConnections()

	probeCtx, cancel := context.WithTimeout(ctx, reconnectProbeTimeout)
	defer cancel()
	req, err := http.NewRequestWithContext(probeCtx, http.MethodHead, target, nil)
	if err != nil {
		return false
	}
	resp, err := tr.RoundTrip(req)
	if err != nil {
		return false
	}
	_ = resp.Body.Close()
	return true
}
