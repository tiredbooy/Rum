package download

import (
	"context"
	"time"
)

// maxConnectivityHosts bounds how many distinct hosts one refresh probes. One
// dead origin must not read as "internet down", so several are tried, but not
// so many that a tick becomes a burst of connections.
const maxConnectivityHosts = 5

// connState is the controller's live view of reachability.
type connState struct {
	offline     bool
	class       ConnClass
	lastProbeAt time.Time
}

// Status reports the last measured reachability and when it was measured.
// Before the first probe (or when there is nothing to probe) it reports online
// with a zero time, so the UI shows no banner on no evidence. Safe on nil.
func (c *ResumeController) Status() (online bool, lastProbeAt time.Time) {
	if c == nil {
		return true, time.Time{}
	}
	c.statusMu.Lock()
	defer c.statusMu.Unlock()
	return !c.conn.offline, c.conn.lastProbeAt
}

// ConnState reports the last classification ("online", "site_blocked",
// "offline"). Before any probe it reads "online". Safe on nil.
func (c *ResumeController) ConnState() string {
	if c == nil {
		return string(ConnOnline)
	}
	c.statusMu.Lock()
	defer c.statusMu.Unlock()
	if c.conn.class == "" {
		return string(ConnOnline)
	}
	return string(c.conn.class)
}

// Touch records that the UI just asked for status, i.e. the window is visible.
// It switches on the idle check for demandWindow.
func (c *ResumeController) Touch() {
	if c != nil {
		c.demandAt.Store(c.now().UnixNano())
	}
}

func (c *ResumeController) demanded() bool {
	at := c.demandAt.Load()
	return at != 0 && c.now().Sub(time.Unix(0, at)) < demandWindow
}

// refreshConnectivity probes the hosts of failed and running jobs plus a small
// set of general endpoints (all concurrently, through the engine's proxy) and
// records the classification. With no failed/running job it probes only while
// the window is visible or a download is queued; otherwise state resets to
// online (no evidence).
func (c *ResumeController) refreshConnectivity(ctx context.Context) {
	if c.manager == nil {
		return
	}
	seen := map[string]bool{}
	var targets []string
	hostOf := map[string]string{}
	queued := false
	for _, job := range c.manager.GetAllJobs() {
		s := job.GetStatus()
		if s == StatusPending {
			queued = true
		}
		if s != StatusError && s != StatusRunning {
			continue
		}
		u := job.GetURL()
		hp, ok := hostPortForURL(u)
		if !ok || seen[hp] || len(targets) == maxConnectivityHosts {
			continue
		}
		seen[hp] = true
		targets = append(targets, u)
		hostOf[hp] = u
	}
	if len(targets) == 0 && !queued && !c.demanded() {
		// No evidence left: drop any stale "offline" so the banner can't stick.
		c.statusMu.Lock()
		c.conn = connState{}
		c.statusMu.Unlock()
		return
	}
	endpoints := probeEndpoints()
	if c.endpoints != nil {
		endpoints = c.endpoints()
	}
	class, blocked := classifyConnectivity(ctx, c.probe, targets, endpoints)
	if ctx.Err() != nil {
		return
	}
	for _, job := range c.manager.GetAllJobs() {
		if job.GetStatus() != StatusError {
			continue
		}
		hp, ok := hostPortForURL(job.GetURL())
		if class == ConnSiteBlocked && ok && blocked[hostOf[hp]] {
			job.markSiteUnreachable()
		} else {
			job.clearSiteUnreachable()
		}
	}
	c.statusMu.Lock()
	c.conn = connState{offline: class == ConnOffline, class: class, lastProbeAt: time.Now()}
	c.statusMu.Unlock()
}
