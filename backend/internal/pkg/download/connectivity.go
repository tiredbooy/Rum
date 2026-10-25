package download

import (
	"context"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/tiredbooy/Rum/backend/internal/pkg/config"
)

// ConnClass is what one connectivity check concluded.
type ConnClass string

const (
	// ConnOnline: the internet works and every probed download host answered.
	ConnOnline ConnClass = "online"
	// ConnSiteBlocked: the internet works but a download's own host does not
	// answer (typically filtered). This is NOT "no internet".
	ConnSiteBlocked ConnClass = "site_blocked"
	// ConnOffline: nothing answered at all.
	ConnOffline ConnClass = "offline"
)

// ErrCodeSiteUnreachable marks a failed download whose host is unreachable
// while the rest of the internet works. Retryable: the controller resumes it
// once the host answers again (e.g. after the user enables a VPN).
const ErrCodeSiteUnreachable = "site_unreachable"

// connectivityDeadline bounds one whole check; all probes run concurrently, so
// the ticker goroutine waits at most this long.
var connectivityDeadline = 5 * time.Second

// demandWindow: GET /status within this long means the window is visible (the
// UI stops polling when hidden), which is when an idle check is worth running.
const demandWindow = 30 * time.Second

// defaultProbeEndpoints mixes domestic and international hosts that stay
// reachable from Iran, so one filtered site cannot read as "no internet".
var defaultProbeEndpoints = []string{
	"https://www.aparat.com",
	"https://www.digikala.com",
	"http://connectivitycheck.gstatic.com/generate_204",
	"https://cp.cloudflare.com/",
	"https://www.bing.com",
}

// probeEndpoints returns the general reachability endpoints: the user's
// settings.json "probe_endpoints" (valid http/https URLs only) or the defaults.
func probeEndpoints() []string {
	var s config.Setting
	if err := s.LoadSettingMetadata(); err == nil {
		var out []string
		for _, raw := range s.ProbeEndpoints {
			raw = strings.TrimSpace(raw)
			if u, err := url.Parse(raw); err == nil && u.Host != "" && (u.Scheme == "http" || u.Scheme == "https") {
				out = append(out, raw)
			}
		}
		if len(out) > 0 {
			return out
		}
	}
	return defaultProbeEndpoints
}

// classifyConnectivity probes every job target and every general endpoint at
// once (bounded by connectivityDeadline) and classifies the result. blocked
// holds the job targets that failed while the internet itself works.
func classifyConnectivity(ctx context.Context, probe func(context.Context, string) bool, jobTargets, endpoints []string) (ConnClass, map[string]bool) {
	ctx, cancel := context.WithTimeout(ctx, connectivityDeadline)
	defer cancel()

	var (
		mu        sync.Mutex
		wg        sync.WaitGroup
		generalOK bool
		jobOK     = map[string]bool{}
		anyJobOK  bool
	)
	for _, t := range endpoints {
		wg.Add(1)
		go func(t string) {
			defer wg.Done()
			if probe(ctx, t) {
				mu.Lock()
				generalOK = true
				mu.Unlock()
			}
		}(t)
	}
	for _, t := range jobTargets {
		wg.Add(1)
		go func(t string) {
			defer wg.Done()
			if probe(ctx, t) {
				mu.Lock()
				jobOK[t] = true
				anyJobOK = true
				mu.Unlock()
			}
		}(t)
	}
	wg.Wait()

	if !generalOK && !anyJobOK {
		return ConnOffline, nil
	}
	blocked := map[string]bool{}
	for _, t := range jobTargets {
		if !jobOK[t] {
			blocked[t] = true
		}
	}
	if len(blocked) > 0 {
		return ConnSiteBlocked, blocked
	}
	return ConnOnline, nil
}

// markSiteUnreachable re-labels a failed job's transient network error as
// "site unreachable". Other failures (disk, TLS, 404...) keep their own code.
func (j *Job) markSiteUnreachable() {
	j.Mu.Lock()
	defer j.Mu.Unlock()
	if j.Status != StatusError || j.Error == nil {
		return
	}
	switch j.ErrorCode {
	case ErrCodeNetworkDown, ErrCodeDNS, ErrCodeTimeout, ErrorCodeStalled:
		j.ErrorCode, j.Retryable = ErrCodeSiteUnreachable, true
	}
}

// clearSiteUnreachable restores the original classification once the host (or
// the whole network) is no longer in the site-blocked state.
func (j *Job) clearSiteUnreachable() {
	j.Mu.Lock()
	defer j.Mu.Unlock()
	if j.ErrorCode == ErrCodeSiteUnreachable && j.Error != nil {
		j.ErrorCode, j.Retryable = ClassifyJobError(j.Error)
	}
}
