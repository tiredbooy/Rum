package download

import (
	"context"
	"sync"
	"time"

	"github.com/tiredbooy/Rum/backend/internal/pkg/config"
)

// nightTickInterval is how often the NightController re-evaluates the window and
// the data cap. Short enough that a cap overshoots by seconds, not minutes.
const nightTickInterval = 10 * time.Second

// nightCapBytes converts the cap setting (GB, binary) to bytes.
func nightCapBytes(gb float64) int64 { return int64(gb * (1 << 30)) }

// nightWindowAt evaluates the night window against local wall-clock time. When
// now is inside a window it returns (true, thatWindowsOpen, itsClose); otherwise
// (false, nextOpen, itsClose). An invalid plan is never active.
//
// Open and close are built with time.Date from calendar fields in now's own
// location, so a DST change shortens or lengthens the window in real time but
// keeps it anchored to the local clock (23:00-06:00 stays 23:00-06:00). A
// start/end pair that has no distinct instants in a DST gap yields no window
// for that day. The window is [open, close): open inclusive, close exclusive.
func nightWindowAt(p config.NightPlan, now time.Time) (active bool, open, closeT time.Time) {
	sm, okS := config.ParseClock(p.Start)
	em, okE := config.ParseClock(p.End)
	if !okS || !okE || sm == em {
		return false, time.Time{}, time.Time{}
	}
	loc := now.Location()
	y, mo, d := now.Date()
	endShift := 0
	if em < sm {
		endShift = 1 // crosses midnight
	}

	var nextOpen, nextClose time.Time
	// Yesterday's window may still be open; today's and tomorrow's are upcoming.
	for off := -1; off <= 1; off++ {
		o := time.Date(y, mo, d+off, sm/60, sm%60, 0, 0, loc)
		c := time.Date(y, mo, d+off+endShift, em/60, em%60, 0, 0, loc)
		if !c.After(o) {
			continue
		}
		if !now.Before(o) && now.Before(c) {
			return true, o, c
		}
		if o.After(now) && (nextOpen.IsZero() || o.Before(nextOpen)) {
			nextOpen, nextClose = o, c
		}
	}
	return false, nextOpen, nextClose
}

// nightPlanSnapshot returns the live night plan.
func (m *JobManager) nightPlanSnapshot() config.NightPlan {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.config.NightPlan
}

// NightHolds reports whether job must not run right now because the night plan
// owns it: the plan is on, the job is in the night queue, and the window is
// closed or its data cap is spent. Automatic starters (resume-on-launch,
// reconnect, the disk watcher, auto-start on add) consult it so they never run a
// night job outside its window. An explicit user start is not gated.
func (m *JobManager) NightHolds(job *Job, now time.Time) bool {
	if !job.GetNightQueue() {
		return false
	}
	plan := m.nightPlanSnapshot()
	if !plan.Enabled {
		return false
	}
	active, _, _ := nightWindowAt(plan, now)
	return !active || m.nightCapHit.Load()
}

// NightStatus is the live state of the night plan for the UI.
type NightStatus struct {
	Active     bool      `json:"active"`
	CapReached bool      `json:"cap_reached"`
	NextStart  time.Time `json:"next_start"` // open of the current or next window
}

// NightStatus reports whether the window is open now, whether its cap is spent,
// and when the current/next window opens.
func (m *JobManager) NightStatus(now time.Time) NightStatus {
	plan := m.nightPlanSnapshot()
	active, open, _ := nightWindowAt(plan, now)
	return NightStatus{
		Active:     plan.Enabled && active,
		CapReached: plan.Enabled && active && m.nightCapHit.Load(),
		NextStart:  open,
	}
}

// parkNightJob pauses a night job for the plan's own reasons (window closed or
// cap spent) and flags it so the next window restarts it. Pausing keeps the
// partial data. Only running and queued jobs are affected.
func (m *JobManager) parkNightJob(job *Job) {
	switch job.GetStatus() {
	case StatusRunning:
		job.SetNightPaused(true)
		_ = m.PauseJob(job.ID)
	case StatusPending:
		m.mu.Lock()
		if job.GetStatus() == StatusPending {
			job.SetStatus(StatusPaused)
			job.SetNightPaused(true)
		}
		m.mu.Unlock()
	default:
		return
	}
	m.requestSave()
}

// SetNightQueue moves jobs into (enabled) or out of the night queue and returns
// how many existing, unfinished jobs it updated. Moving a job that may not run
// right now parks it for the window; releasing one returns it to the normal
// queue without starting it.
func (m *JobManager) SetNightQueue(ids []string, enabled bool, now time.Time) int {
	plan := m.nightPlanSnapshot()
	held := false
	if plan.Enabled {
		active, _, _ := nightWindowAt(plan, now)
		held = !active || m.nightCapHit.Load()
	}
	updated := 0
	for _, id := range ids {
		job, ok := m.GetJob(id)
		if !ok || job.GetStatus() == StatusCompleted {
			continue
		}
		updated++
		if !enabled {
			job.SetNightQueue(false)
			job.SetNightPaused(false)
			continue
		}
		job.SetNightQueue(true)
		switch job.GetStatus() {
		case StatusPaused:
			job.SetNightPaused(true) // wait for tonight instead of staying idle
		case StatusRunning, StatusPending:
			if held {
				m.parkNightJob(job)
			}
		}
	}
	m.requestSave()
	return updated
}

// NightController applies the night plan: it starts night jobs when the window
// opens (or at launch inside it), parks them when it closes, and enforces the
// per-window data cap. It owns no other job state: error handling, reconnect and
// disk-full recovery stay with their own controllers, which ask NightHolds
// before restarting a night job.
type NightController struct {
	manager  *JobManager
	interval time.Duration

	// Ticker-goroutine state.
	wasActive bool
	winOpen   time.Time        // open time of the window being accounted
	used      int64            // night bytes downloaded in this window
	last      map[string]int64 // Downloaded per night job at the previous tick

	cancel context.CancelFunc
	wg     sync.WaitGroup
	once   sync.Once
}

// NewNightController builds a controller bound to manager. The plan is read from
// the manager's live settings every tick, so edits apply without a restart.
func NewNightController(manager *JobManager) *NightController {
	return &NightController{manager: manager, interval: nightTickInterval, last: map[string]int64{}}
}

// Start applies the plan once immediately (so launching inside the window starts
// night jobs at once) and then every interval until ctx ends or Stop is called.
func (c *NightController) Start(ctx context.Context) {
	runCtx, cancel := context.WithCancel(ctx)
	c.cancel = cancel
	c.wg.Add(1)
	go func() {
		defer c.wg.Done()
		c.tick(runCtx, time.Now())
		ticker := time.NewTicker(c.interval)
		defer ticker.Stop()
		for {
			select {
			case <-runCtx.Done():
				return
			case <-ticker.C:
				c.tick(runCtx, time.Now())
			}
		}
	}()
}

// Stop cancels the ticker goroutine and waits for it. Idempotent.
func (c *NightController) Stop() {
	c.once.Do(func() {
		if c.cancel != nil {
			c.cancel()
		}
		c.wg.Wait()
	})
}

// tick evaluates the plan at now.
func (c *NightController) tick(ctx context.Context, now time.Time) {
	m := c.manager
	if m == nil {
		return
	}
	plan := m.nightPlanSnapshot()
	if !plan.Enabled {
		m.nightCapHit.Store(false)
		c.wasActive, c.winOpen = false, time.Time{}
		// With the plan off, night jobs held pending are plain queued downloads:
		// release them so turning the plan off (or launching with it off) never
		// strands them. Jobs waiting on their own scheduled start are left alone.
		for _, j := range c.nightJobs() {
			if j.GetStatus() == StatusPending && !j.GetStartAt().After(now) {
				_ = m.StartJob(ctx, j.ID)
			}
		}
		return
	}
	active, open, _ := nightWindowAt(plan, now)
	all := c.nightJobs()
	jobs := make([]*Job, 0, len(all))
	for _, j := range all {
		if j.GetStatus() != StatusCompleted {
			jobs = append(jobs, j)
		}
	}

	if !active {
		// Pause only on the open -> closed transition, so a night job the user
		// starts by hand outside the window is never fought.
		if c.wasActive && plan.PauseAtClose {
			for _, j := range jobs {
				m.parkNightJob(j)
			}
		}
		m.nightCapHit.Store(false)
		c.wasActive, c.winOpen = false, time.Time{}
		return
	}

	if !c.winOpen.Equal(open) { // a new window (or the first one observed)
		c.winOpen, c.used, c.last = open, 0, map[string]int64{}
		m.nightCapHit.Store(false)
	}
	c.account(all) // finished jobs still count for bytes moved since the last tick

	if plan.CapGB > 0 && c.used >= nightCapBytes(plan.CapGB) {
		if !m.nightCapHit.Swap(true) {
			for _, j := range jobs {
				m.parkNightJob(j)
			}
		}
		c.wasActive = true
		return
	}
	c.wasActive = true

	if !plan.StartAtOpen {
		return
	}
	for _, j := range jobs {
		if ctx.Err() != nil {
			return
		}
		st := j.GetStatus()
		if st == StatusPending || (st == StatusPaused && j.GetNightPaused()) {
			_ = m.StartJob(ctx, j.ID)
		}
	}
}

// nightJobs returns every night-queue job, finished or not.
func (c *NightController) nightJobs() []*Job {
	var out []*Job
	for _, j := range c.manager.GetAllJobs() {
		if j.GetNightQueue() {
			out = append(out, j)
		}
	}
	return out
}

// account adds the bytes night jobs downloaded since the previous tick to the
// window's total. A job first seen in a window contributes from that point on.
// Usage is in memory: restarting the app mid-window starts the count again.
func (c *NightController) account(jobs []*Job) {
	seen := make(map[string]int64, len(jobs))
	for _, j := range jobs {
		cur := j.GetDownloaded()
		if prev, ok := c.last[j.ID]; ok && cur > prev {
			c.used += cur - prev
		}
		seen[j.ID] = cur
	}
	c.last = seen
}
