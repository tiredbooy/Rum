package download

import (
	"context"
	"sync"
	"time"

	"github.com/tiredbooy/Rum/backend/internal/pkg/config"
)

// scheduleTickInterval is how often the controller re-evaluates the active
// bandwidth window and starts due scheduled jobs. 30s is responsive enough for
// hour-granular windows without busy work.
const scheduleTickInterval = 30 * time.Second

// ruleMatches reports whether a bandwidth rule is active at time now. It checks
// the day-of-week restriction (empty Days = every day) and the hour window,
// supporting wrap-around windows where StartHour > EndHour (e.g. 22:00-06:00).
//
// The window is inclusive of StartHour and exclusive of EndHour at hour
// granularity: a rule {Start:0,End:8} matches hours 0..7. A rule where
// Start==End is treated as "all day".
func ruleMatches(r config.SpeedRule, now time.Time) bool {
	if len(r.Days) > 0 {
		wd := int(now.Weekday()) // 0=Sun..6=Sat
		found := false
		for _, d := range r.Days {
			if d == wd {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}

	h := now.Hour()
	start, end := r.StartHour, r.EndHour
	switch {
	case start == end:
		return true // all-day window
	case start < end:
		return h >= start && h < end
	default:
		// Wrap-around: active from start..23 and 0..end-1.
		return h >= start || h < end
	}
}

// activeLimitKBps returns the bandwidth limit (KB/s) that applies at time now,
// given the configured rules and a fallback used when no rule is active. Among
// all rules active at now, the most restrictive non-zero limit wins (a smaller
// KB/s is more restrictive). Rules with LimitKBps == 0 mean "unlimited within
// this window" and never tighten the cap; if the only active rules are
// unlimited, the fallback is returned. This function is pure and deterministic.
func activeLimitKBps(rules []config.SpeedRule, now time.Time, fallback int) int {
	best := -1 // sentinel: no non-zero active rule seen yet
	for _, r := range rules {
		if r.LimitKBps <= 0 {
			continue // unlimited window: does not tighten
		}
		if !ruleMatches(r, now) {
			continue
		}
		if best == -1 || r.LimitKBps < best {
			best = r.LimitKBps
		}
	}
	if best == -1 {
		return fallback
	}
	return best
}

// ScheduleController periodically applies the active bandwidth window to the
// shared SpeedGovernor (and the manager's stored limit) and, when scheduled
// start is enabled, starts pending jobs whose StartAt is due. It runs a single
// context-cancelable ticker goroutine; Stop cancels it and waits for it to exit,
// so it is leak-free.
type ScheduleController struct {
	manager  *JobManager
	governor *SpeedGovernor
	setting  config.Setting

	cancel context.CancelFunc
	wg     sync.WaitGroup
	once   sync.Once
}

// NewScheduleController builds a controller bound to the given manager and
// governor. setting is a snapshot used for the schedule rules; the controller
// re-reads the latest settings from disk each tick so live edits via the
// settings API are honored.
func NewScheduleController(manager *JobManager, governor *SpeedGovernor, setting config.Setting) *ScheduleController {
	return &ScheduleController{
		manager:  manager,
		governor: governor,
		setting:  setting,
	}
}

// Start launches the ticker goroutine. It applies the current window once
// immediately so the cap is correct from the outset, then re-applies every
// scheduleTickInterval until ctx is cancelled or Stop is called. Calling Start
// more than once is safe but only the first call has effect.
func (c *ScheduleController) Start(ctx context.Context) {
	runCtx, cancel := context.WithCancel(ctx)
	c.cancel = cancel

	c.wg.Add(1)
	go func() {
		defer c.wg.Done()
		// Apply immediately so we don't wait a full interval for the first window.
		c.tick(runCtx)

		ticker := time.NewTicker(scheduleTickInterval)
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

// tick re-reads settings, applies the active bandwidth limit to the governor and
// manager, and starts any due scheduled jobs.
func (c *ScheduleController) tick(ctx context.Context) {
	// Re-read settings so runtime edits (via the schedule/settings endpoints) take
	// effect. Fall back to the snapshot if the read fails.
	setting := c.setting
	var fresh config.Setting
	if err := fresh.LoadSettingMetadata(); err == nil {
		setting = fresh
	}

	now := time.Now()
	limit := activeLimitKBps(setting.BandwidthSchedule, now, setting.SpeedLimitKB)
	if c.governor != nil {
		c.governor.SetLimitKBps(limit)
	}
	if c.manager != nil {
		c.manager.SetSpeedLimit(limit)
	}

	if setting.ScheduledStartEnabled && c.manager != nil {
		c.startDueJobs(ctx, now)
	}
}

// startDueJobs starts pending jobs whose StartAt is set and has passed. StartAt
// is carried on the manager's per-job options; today the engine stores it on the
// job's Options snapshot, so we start any pending job whose StartAt (if any) is
// due. Jobs with a zero StartAt are left to the normal (immediate) start flow.
func (c *ScheduleController) startDueJobs(ctx context.Context, now time.Time) {
	for _, job := range c.manager.GetAllJobs() {
		if job.GetStatus() != StatusPending {
			continue
		}
		sa := job.GetStartAt()
		if sa.IsZero() || sa.After(now) {
			continue
		}
		_ = c.manager.StartJob(ctx, job.ID)
	}
}

// Stop cancels the ticker goroutine and waits for it to exit. It is idempotent.
func (c *ScheduleController) Stop() {
	c.once.Do(func() {
		if c.cancel != nil {
			c.cancel()
		}
		c.wg.Wait()
	})
}
