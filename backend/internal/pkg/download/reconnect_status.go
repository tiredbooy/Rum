package download

import (
	"context"
	"time"
)

// maxConnectivityHosts bounds how many distinct hosts one refresh probes. One
// dead origin must not read as "internet down", so several are tried, but not
// so many that a tick becomes a burst of connections.
const maxConnectivityHosts = 3

// connState is the controller's live view of reachability.
type connState struct {
	offline     bool
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

// refreshConnectivity probes the hosts of failed and running jobs (through the
// same proxy as the engine) and records whether ANY answered. With no probeable
// job the state resets to online (no evidence).
func (c *ResumeController) refreshConnectivity(ctx context.Context) {
	if c.manager == nil {
		return
	}
	seen := map[string]bool{}
	var targets []string
	for _, job := range c.manager.GetAllJobs() {
		if s := job.GetStatus(); s != StatusError && s != StatusRunning {
			continue
		}
		u := job.GetURL()
		hp, ok := hostPortForURL(u)
		if !ok || seen[hp] {
			continue
		}
		seen[hp] = true
		targets = append(targets, u)
		if len(targets) == maxConnectivityHosts {
			break
		}
	}
	if len(targets) == 0 {
		// No evidence left: drop any stale "offline" so the banner can't stick.
		c.statusMu.Lock()
		c.conn = connState{}
		c.statusMu.Unlock()
		return
	}
	reachable := false
	for _, u := range targets {
		if ctx.Err() != nil {
			return
		}
		if c.probe(ctx, u) {
			reachable = true
			break
		}
	}
	c.statusMu.Lock()
	c.conn = connState{offline: !reachable, lastProbeAt: time.Now()}
	c.statusMu.Unlock()
}
