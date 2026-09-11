package download

import (
	"context"
	"log"
	"net"
	"net/url"
	"sync"
	"time"

	"github.com/tiredbooy/Rum/backend/internal/pkg/config"
)

// reconnectTickInterval is how often the controller re-checks failed downloads.
// 20s is responsive enough to feel automatic after a Wi-Fi drop without turning
// a long outage into a connection-attempt storm.
const reconnectTickInterval = 20 * time.Second

// maxReconnectAttempts bounds how many times a single job is auto-resumed by
// this controller. A download that keeps failing the moment the host is
// reachable is not a connectivity problem, and retrying it forever would hide a
// real error from the user.
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

	// probe reports whether addr (host:port) is reachable. Injectable so tests
	// don't touch the network.
	probe func(ctx context.Context, addr string) bool

	// attempts counts auto-resumes per job id, so one permanently-broken download
	// cannot be retried forever. Only touched from the ticker goroutine.
	attempts map[string]int

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
		if job.GetStatus() != StatusError {
			// Recovered (or was restarted by hand): forget its attempt count so a
			// later outage gets a fresh budget.
			delete(c.attempts, id)
			continue
		}
		if !isRetryableError(job.GetError()) {
			continue // a 404 / checksum mismatch is not a connectivity problem
		}
		if c.attempts[id] >= maxReconnectAttempts {
			continue
		}
		addr, ok := hostPortForURL(job.GetURL())
		if !ok || !c.probe(ctx, addr) {
			continue // still offline (or an unusable URL) — try again next tick
		}

		c.attempts[id]++
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

// dialProbe reports whether addr accepts a TCP connection. Probing the download's
// OWN host (rather than some third-party "are we online" endpoint) means the
// check answers the question that actually matters and sends no traffic anywhere
// the user was not already downloading from.
func dialProbe(ctx context.Context, addr string) bool {
	dialer := &net.Dialer{Timeout: reconnectProbeTimeout}
	probeCtx, cancel := context.WithTimeout(ctx, reconnectProbeTimeout)
	defer cancel()
	conn, err := dialer.DialContext(probeCtx, "tcp", addr)
	if err != nil {
		return false
	}
	_ = conn.Close()
	return true
}
