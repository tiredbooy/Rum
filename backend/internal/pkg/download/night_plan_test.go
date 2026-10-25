package download

import (
	"context"
	"sync/atomic"
	"testing"
	"time"
	_ "time/tzdata"

	"github.com/tiredbooy/Rum/backend/internal/pkg/config"
)

func nightPlan(start, end string) config.NightPlan {
	return config.NightPlan{Enabled: true, Start: start, End: end, StartAtOpen: true, PauseAtClose: true}
}

func at(t *testing.T, loc *time.Location, y int, mo time.Month, d, h, mi int) time.Time {
	t.Helper()
	return time.Date(y, mo, d, h, mi, 0, 0, loc)
}

func TestNightWindowAt(t *testing.T) {
	utc := time.UTC
	cases := []struct {
		name       string
		plan       config.NightPlan
		now        time.Time
		wantActive bool
		wantOpen   time.Time
		wantClose  time.Time
	}{
		{"inside same-day", nightPlan("02:00", "07:00"), at(t, utc, 2025, 1, 10, 3, 0), true,
			at(t, utc, 2025, 1, 10, 2, 0), at(t, utc, 2025, 1, 10, 7, 0)},
		{"one second before open", nightPlan("02:00", "07:00"), at(t, utc, 2025, 1, 10, 1, 59).Add(59 * time.Second), false,
			at(t, utc, 2025, 1, 10, 2, 0), at(t, utc, 2025, 1, 10, 7, 0)},
		{"open edge is inclusive", nightPlan("02:00", "07:00"), at(t, utc, 2025, 1, 10, 2, 0), true,
			at(t, utc, 2025, 1, 10, 2, 0), at(t, utc, 2025, 1, 10, 7, 0)},
		{"close edge is exclusive", nightPlan("02:00", "07:00"), at(t, utc, 2025, 1, 10, 7, 0), false,
			at(t, utc, 2025, 1, 11, 2, 0), at(t, utc, 2025, 1, 11, 7, 0)},
		{"after close next is tomorrow", nightPlan("02:00", "07:00"), at(t, utc, 2025, 1, 10, 12, 0), false,
			at(t, utc, 2025, 1, 11, 2, 0), at(t, utc, 2025, 1, 11, 7, 0)},
		{"cross-midnight evening side", nightPlan("23:00", "06:00"), at(t, utc, 2025, 1, 10, 23, 30), true,
			at(t, utc, 2025, 1, 10, 23, 0), at(t, utc, 2025, 1, 11, 6, 0)},
		{"cross-midnight morning side", nightPlan("23:00", "06:00"), at(t, utc, 2025, 1, 11, 0, 30), true,
			at(t, utc, 2025, 1, 10, 23, 0), at(t, utc, 2025, 1, 11, 6, 0)},
		{"cross-midnight last minute", nightPlan("23:00", "06:00"), at(t, utc, 2025, 1, 11, 5, 59), true,
			at(t, utc, 2025, 1, 10, 23, 0), at(t, utc, 2025, 1, 11, 6, 0)},
		{"cross-midnight closed at 06:00", nightPlan("23:00", "06:00"), at(t, utc, 2025, 1, 11, 6, 0), false,
			at(t, utc, 2025, 1, 11, 23, 0), at(t, utc, 2025, 1, 12, 6, 0)},
		{"cross-midnight midday", nightPlan("23:00", "06:00"), at(t, utc, 2025, 1, 11, 12, 0), false,
			at(t, utc, 2025, 1, 11, 23, 0), at(t, utc, 2025, 1, 12, 6, 0)},
		{"month boundary", nightPlan("23:00", "06:00"), at(t, utc, 2025, 2, 1, 1, 0), true,
			at(t, utc, 2025, 1, 31, 23, 0), at(t, utc, 2025, 2, 1, 6, 0)},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			active, open, closeT := nightWindowAt(tc.plan, tc.now)
			if active != tc.wantActive || !open.Equal(tc.wantOpen) || !closeT.Equal(tc.wantClose) {
				t.Fatalf("got active=%v open=%v close=%v\nwant active=%v open=%v close=%v",
					active, open, closeT, tc.wantActive, tc.wantOpen, tc.wantClose)
			}
		})
	}
}

func TestNightWindowAtInvalidPlanIsNeverActive(t *testing.T) {
	for _, p := range []config.NightPlan{
		{Enabled: true, Start: "", End: ""},
		{Enabled: true, Start: "02:00", End: "02:00"},
		{Enabled: true, Start: "xx", End: "07:00"},
	} {
		if active, _, _ := nightWindowAt(p, time.Now()); active {
			t.Errorf("invalid plan %+v reported active", p)
		}
	}
}

// DST: the window is anchored to local wall-clock times, so on a spring-forward
// night the 23:00-06:00 window really lasts 6 hours and the close time is the
// local 06:00 (EDT), not a fixed 7h offset from the open.
func TestNightWindowAtAcrossDST(t *testing.T) {
	ny, err := time.LoadLocation("America/New_York")
	if err != nil {
		t.Skipf("tzdata unavailable: %v", err)
	}
	plan := nightPlan("23:00", "06:00")

	now := at(t, ny, 2024, 3, 10, 5, 0) // EDT, after the 02:00 jump
	active, open, closeT := nightWindowAt(plan, now)
	if !active {
		t.Fatalf("05:00 EDT should be inside 23:00-06:00")
	}
	if !open.Equal(at(t, ny, 2024, 3, 9, 23, 0)) || !closeT.Equal(at(t, ny, 2024, 3, 10, 6, 0)) {
		t.Fatalf("open=%v close=%v", open, closeT)
	}
	if got := closeT.Sub(open); got != 6*time.Hour {
		t.Fatalf("spring-forward window lasts %v, want 6h of real time", got)
	}
	if active, _, _ := nightWindowAt(plan, at(t, ny, 2024, 3, 10, 6, 0)); active {
		t.Fatalf("06:00 EDT must be closed")
	}

	// Fall back: 01:30 happens twice; a 00:30-03:00 window covers both.
	fall := nightPlan("00:30", "03:00")
	first := time.Date(2024, 11, 3, 5, 45, 0, 0, time.UTC).In(ny) // 01:45 EDT
	second := first.Add(time.Hour)                                // 01:45 EST
	for _, n := range []time.Time{first, second} {
		if active, _, _ := nightWindowAt(fall, n); !active {
			t.Fatalf("%v should be inside 00:30-03:00", n)
		}
	}
	active, _, closeT = nightWindowAt(fall, second)
	if !active || closeT.Hour() != 3 || closeT.Minute() != 0 {
		t.Fatalf("close = %v, want local 03:00", closeT)
	}
}

// --- controller behaviour ------------------------------------------------

func newNightManager(t *testing.T, plan config.NightPlan) *JobManager {
	t.Helper()
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	m := newQuietManager(t)
	setNightPlan(m, plan)
	return m
}

func setNightPlan(m *JobManager, plan config.NightPlan) {
	m.mu.Lock()
	m.config.NightPlan = plan
	m.mu.Unlock()
}

func addNightJob(m *JobManager, id, status string) *Job {
	j := &Job{ID: id, URL: "https://example.test/" + id, Status: status}
	j.SetNightQueue(true)
	m.mu.Lock()
	m.jobs[id] = j
	m.mu.Unlock()
	return j
}

func addPlainJob(m *JobManager, id, status string) *Job {
	j := &Job{ID: id, URL: "https://example.test/" + id, Status: status}
	m.mu.Lock()
	m.jobs[id] = j
	m.mu.Unlock()
	return j
}

var (
	nightIn  = time.Date(2025, 1, 10, 3, 0, 0, 0, time.UTC)
	nightOut = time.Date(2025, 1, 10, 12, 0, 0, 0, time.UTC)
)

// Launch outside the window: an interrupted night job stays paused (flagged for
// the window) while an ordinary interrupted job still resumes.
func TestResumeOnLaunchKeepsNightJobsPausedOutsideWindow(t *testing.T) {
	m := newNightManager(t, nightPlan("02:00", "07:00"))
	night := addNightJob(m, "n", StatusPaused)
	night.SetInterrupted(true)
	plain := addPlainJob(m, "p", StatusPaused)
	plain.SetInterrupted(true)

	if got := m.ResumeInterruptedJobs(context.Background()); got != 1 {
		t.Fatalf("resumed = %d, want 1 (only the non-night job)", got)
	}
	if m.sched.Queued() != 1 {
		t.Fatalf("queued = %d, want only the plain job", m.sched.Queued())
	}
	if night.GetStatus() != StatusPaused || !night.GetNightPaused() || night.GetInterrupted() {
		t.Fatalf("night job: status=%s nightPaused=%v interrupted=%v", night.GetStatus(), night.GetNightPaused(), night.GetInterrupted())
	}

	// Outside the window the controller leaves it alone.
	c := NewNightController(m)
	c.tick(context.Background(), nightOut)
	if night.GetStatus() != StatusPaused || m.sched.Queued() != 1 {
		t.Fatalf("night job moved outside the window (status=%s queued=%d)", night.GetStatus(), m.sched.Queued())
	}
}

// Launch inside the window: the same flagged job is started by the first tick.
func TestNightControllerStartsJobsWhenAppLaunchesInsideWindow(t *testing.T) {
	m := newNightManager(t, nightPlan("02:00", "07:00"))
	night := addNightJob(m, "n", StatusPaused)
	night.SetInterrupted(true)
	m.ResumeInterruptedJobs(context.Background())
	if m.sched.Queued() != 0 {
		t.Fatalf("resume-on-launch queued a night job itself")
	}

	NewNightController(m).tick(context.Background(), nightIn)
	if m.sched.Queued() != 1 || night.GetNightPaused() {
		t.Fatalf("queued=%d nightPaused=%v, want started with the flag cleared", m.sched.Queued(), night.GetNightPaused())
	}
}

func TestResumeOnLaunchTreatsNightJobsNormallyWhenPlanDisabled(t *testing.T) {
	p := nightPlan("02:00", "07:00")
	p.Enabled = false
	m := newNightManager(t, p)
	night := addNightJob(m, "n", StatusPaused)
	night.SetInterrupted(true)
	if got := m.ResumeInterruptedJobs(context.Background()); got != 1 {
		t.Fatalf("resumed = %d, want 1 with the plan off", got)
	}
}

// Window open starts queued (pending) and schedule-paused night jobs only.
func TestNightControllerOnlyStartsNightQueueJobs(t *testing.T) {
	m := newNightManager(t, nightPlan("02:00", "07:00"))
	pending := addNightJob(m, "pending", StatusPending)
	sched := addNightJob(m, "schedpaused", StatusPaused)
	sched.SetNightPaused(true)
	userPaused := addNightJob(m, "userpaused", StatusPaused)
	plainPaused := addPlainJob(m, "plain", StatusPaused)
	plainPaused.SetNightPaused(true) // stray flag on a non-night job: ignored
	done := addNightJob(m, "done", StatusCompleted)

	NewNightController(m).tick(context.Background(), nightIn)

	if m.sched.Queued() != 2 {
		t.Fatalf("queued = %d, want pending + schedule-paused only", m.sched.Queued())
	}
	if pending.GetStatus() != StatusPending || sched.GetStatus() != StatusPending {
		t.Fatalf("started jobs should be queued as pending")
	}
	for _, j := range []*Job{userPaused, plainPaused, done} {
		if j.GetStatus() == StatusPending {
			t.Fatalf("job %s was started but must not be", j.ID)
		}
	}
}

func TestNightControllerStartAtOpenOffDoesNotStart(t *testing.T) {
	p := nightPlan("02:00", "07:00")
	p.StartAtOpen = false
	m := newNightManager(t, p)
	addNightJob(m, "n", StatusPending)
	NewNightController(m).tick(context.Background(), nightIn)
	if m.sched.Queued() != 0 {
		t.Fatalf("queued = %d, want 0 with start_at_open off", m.sched.Queued())
	}
}

func TestNightControllerDisabledPlanDoesNothing(t *testing.T) {
	p := nightPlan("02:00", "07:00")
	p.Enabled = false
	m := newNightManager(t, p)
	addNightJob(m, "n", StatusPaused)
	NewNightController(m).tick(context.Background(), nightIn)
	if m.sched.Queued() != 0 {
		t.Fatalf("disabled plan started a job")
	}
}

// Turning the plan off (or launching with it off) must not strand night jobs
// that were held pending: they run like any queued download.
func TestNightControllerDisabledPlanReleasesHeldPendingJobs(t *testing.T) {
	p := nightPlan("02:00", "07:00")
	p.Enabled = false
	m := newNightManager(t, p)
	held := addNightJob(m, "held", StatusPending)
	future := addNightJob(m, "future", StatusPending)
	future.SetStartAt(nightOut.Add(24 * time.Hour))
	NewNightController(m).tick(context.Background(), nightOut)
	if m.sched.Queued() != 1 || held.GetStatus() != StatusPending {
		t.Fatalf("queued = %d, want only the held job released", m.sched.Queued())
	}
}

// Quitting with the plan off must flag a queued night job for resume-on-launch.
func TestGracefulShutdownFlagsQueuedNightJobWhenPlanDisabled(t *testing.T) {
	p := nightPlan("02:00", "07:00")
	p.Enabled = false
	m := newNightManager(t, p)
	j := addNightJob(m, "n", StatusPending)
	m.GracefulShutdown(context.Background())
	if j.GetStatus() != StatusPaused || !j.GetInterrupted() || j.GetNightPaused() {
		t.Fatalf("status=%s interrupted=%v nightPaused=%v", j.GetStatus(), j.GetInterrupted(), j.GetNightPaused())
	}
}

func runningJob(m *JobManager, id string, night bool) (*Job, *atomic.Bool) {
	var j *Job
	if night {
		j = addNightJob(m, id, StatusRunning)
	} else {
		j = addPlainJob(m, id, StatusRunning)
	}
	cancelled := &atomic.Bool{}
	j.SetCancelFunc(func() { cancelled.Store(true) })
	return j, cancelled
}

// Closing the window pauses running night jobs (cancel keeps partial data) and
// queued ones, flags them so the next window restarts them, and leaves
// everything else running.
func TestNightControllerPausesAtWindowEnd(t *testing.T) {
	m := newNightManager(t, nightPlan("02:00", "07:00"))
	night, nightCancelled := runningJob(m, "n", true)
	plain, plainCancelled := runningJob(m, "p", false)
	queued := addNightJob(m, "q", StatusPending)

	c := NewNightController(m)
	c.tick(context.Background(), nightIn) // window open: observed active
	if nightCancelled.Load() {
		t.Fatalf("paused inside the window")
	}
	c.tick(context.Background(), nightOut) // window closed

	if !nightCancelled.Load() || !night.GetNightPaused() {
		t.Fatalf("running night job: cancelled=%v nightPaused=%v", nightCancelled.Load(), night.GetNightPaused())
	}
	if plainCancelled.Load() || plain.GetNightPaused() {
		t.Fatalf("non-night job was touched")
	}
	if queued.GetStatus() != StatusPaused || !queued.GetNightPaused() {
		t.Fatalf("queued night job: status=%s nightPaused=%v", queued.GetStatus(), queued.GetNightPaused())
	}
}

func TestNightControllerPauseAtCloseOffKeepsRunning(t *testing.T) {
	p := nightPlan("02:00", "07:00")
	p.PauseAtClose = false
	m := newNightManager(t, p)
	_, cancelled := runningJob(m, "n", true)
	c := NewNightController(m)
	c.tick(context.Background(), nightIn)
	c.tick(context.Background(), nightOut)
	if cancelled.Load() {
		t.Fatalf("paused although pause_at_close is off")
	}
}

// A job the user starts by hand outside the window is not fought: only the
// open -> closed transition pauses.
func TestNightControllerDoesNotFightManualStartOutsideWindow(t *testing.T) {
	m := newNightManager(t, nightPlan("02:00", "07:00"))
	_, cancelled := runningJob(m, "n", true)
	c := NewNightController(m)
	c.tick(context.Background(), nightOut)
	c.tick(context.Background(), nightOut.Add(time.Minute))
	if cancelled.Load() {
		t.Fatalf("controller paused a manually started job outside the window")
	}
}

func TestNightControllerPausedJobRestartsNextWindow(t *testing.T) {
	m := newNightManager(t, nightPlan("02:00", "07:00"))
	night, _ := runningJob(m, "n", true)
	c := NewNightController(m)
	c.tick(context.Background(), nightIn)
	c.tick(context.Background(), nightOut)
	night.SetStatus(StatusPaused) // the engine finishes the pause

	c.tick(context.Background(), nightIn.Add(24*time.Hour))
	if m.sched.Queued() != 1 || night.GetNightPaused() {
		t.Fatalf("next window did not restart the job (queued=%d)", m.sched.Queued())
	}
}

// --- data cap ------------------------------------------------------------

func TestNightCapPausesJobsAndBlocksRestart(t *testing.T) {
	p := nightPlan("02:00", "07:00")
	p.CapGB = 1
	m := newNightManager(t, p)
	j, cancelled := runningJob(m, "n", true)
	c := NewNightController(m)

	c.tick(context.Background(), nightIn)
	j.SetDownloaded(nightCapBytes(1) / 2)
	c.tick(context.Background(), nightIn.Add(time.Minute))
	if cancelled.Load() || m.NightHolds(j, nightIn.Add(time.Minute)) {
		t.Fatalf("half the cap spent must not stop anything")
	}

	j.SetDownloaded(nightCapBytes(1) + 10)
	c.tick(context.Background(), nightIn.Add(2*time.Minute))
	if !cancelled.Load() || !j.GetNightPaused() {
		t.Fatalf("cap reached but job not paused (cancelled=%v)", cancelled.Load())
	}
	if !m.NightHolds(j, nightIn.Add(2*time.Minute)) {
		t.Fatalf("a night job must be held once the cap is spent")
	}

	// Still inside the window: it must not be restarted.
	j.SetStatus(StatusPaused)
	c.tick(context.Background(), nightIn.Add(3*time.Minute))
	if m.sched.Queued() != 0 {
		t.Fatalf("job restarted after the cap was reached")
	}

	// The next window gets a fresh allowance.
	next := nightIn.Add(24 * time.Hour)
	c.tick(context.Background(), next)
	if m.sched.Queued() != 1 || m.NightHolds(j, next) {
		t.Fatalf("new window did not reset the cap (queued=%d)", m.sched.Queued())
	}
}

func TestNightCapCountsAllNightJobsNotPlainOnes(t *testing.T) {
	p := nightPlan("02:00", "07:00")
	p.CapGB = 1
	m := newNightManager(t, p)
	a, aCancel := runningJob(m, "a", true)
	b, bCancel := runningJob(m, "b", true)
	plain, plainCancel := runningJob(m, "p", false)
	c := NewNightController(m)
	c.tick(context.Background(), nightIn)

	half := nightCapBytes(1) / 2
	a.SetDownloaded(half)
	b.SetDownloaded(half + 1)
	plain.SetDownloaded(nightCapBytes(5))
	c.tick(context.Background(), nightIn.Add(time.Minute))

	if !aCancel.Load() || !bCancel.Load() {
		t.Fatalf("two night jobs together spent the cap but were not paused")
	}
	if plainCancel.Load() {
		t.Fatalf("non-night traffic must not count against the night cap")
	}
}

func TestNightNoCapMeansNoLimit(t *testing.T) {
	m := newNightManager(t, nightPlan("02:00", "07:00")) // CapGB == 0
	j, cancelled := runningJob(m, "n", true)
	c := NewNightController(m)
	c.tick(context.Background(), nightIn)
	j.SetDownloaded(nightCapBytes(500))
	c.tick(context.Background(), nightIn.Add(time.Minute))
	if cancelled.Load() {
		t.Fatalf("no cap configured but job was paused")
	}
}

// --- not fighting the other controllers ---------------------------------

func TestReconnectDoesNotResumeNightJobOutsideWindow(t *testing.T) {
	m := newNightManager(t, nightPlan("02:00", "07:00"))
	job := addNightJob(m, "n", StatusError)
	job.SetError(context.DeadlineExceeded)

	rc := NewResumeController(m)
	rc.now = func() time.Time { return nightOut }
	if rc.eligible(job) {
		t.Fatalf("reconnect would restart a night job outside the window")
	}
	rc.now = func() time.Time { return nightIn }
	if !rc.eligible(job) {
		t.Fatalf("reconnect should resume a night job inside the window")
	}
}

func TestNightHoldsOnlyAppliesToNightJobsWithPlanOn(t *testing.T) {
	m := newNightManager(t, nightPlan("02:00", "07:00"))
	night := addNightJob(m, "n", StatusPending)
	plain := addPlainJob(m, "p", StatusPending)
	if !m.NightHolds(night, nightOut) || m.NightHolds(night, nightIn) {
		t.Fatalf("night job hold should be true outside, false inside")
	}
	if m.NightHolds(plain, nightOut) {
		t.Fatalf("plain job held")
	}
	p := nightPlan("02:00", "07:00")
	p.Enabled = false
	setNightPlan(m, p)
	if m.NightHolds(night, nightOut) {
		t.Fatalf("job held while the plan is off")
	}
}

// --- API-facing helpers --------------------------------------------------

func TestSetNightQueueMovesAndReleasesJobs(t *testing.T) {
	m := newNightManager(t, nightPlan("02:00", "07:00"))
	paused := addPlainJob(m, "a", StatusPaused)
	running, cancelled := runningJob(m, "b", false)
	done := addPlainJob(m, "c", StatusCompleted)

	// Outside the window a running job is paused (partial kept) and flagged.
	n := m.SetNightQueue([]string{"a", "b", "c", "missing"}, true, nightOut)
	if n != 2 {
		t.Fatalf("updated = %d, want 2 (completed and unknown ids skipped)", n)
	}
	if !paused.GetNightQueue() || !paused.GetNightPaused() {
		t.Fatalf("paused job not queued for night")
	}
	if !running.GetNightQueue() || !running.GetNightPaused() || !cancelled.Load() {
		t.Fatalf("running job should be paused for tonight")
	}
	if done.GetNightQueue() {
		t.Fatalf("completed job moved")
	}

	// Releasing returns it to the normal queue without starting it.
	m.SetNightQueue([]string{"a"}, false, nightOut)
	if paused.GetNightQueue() || paused.GetNightPaused() {
		t.Fatalf("job still in the night queue")
	}
}

func TestSetNightQueueInsideWindowKeepsRunningJobsRunning(t *testing.T) {
	m := newNightManager(t, nightPlan("02:00", "07:00"))
	_, cancelled := runningJob(m, "b", false)
	m.SetNightQueue([]string{"b"}, true, nightIn)
	if cancelled.Load() {
		t.Fatalf("paused a job that may run right now")
	}
}

func TestNightJobFlagsRoundTripThroughJSON(t *testing.T) {
	j := &Job{ID: "x", URL: "https://example.test/x", Status: StatusPaused}
	j.SetNightQueue(true)
	j.SetNightPaused(true)
	raw, err := j.MarshalJSON()
	if err != nil {
		t.Fatal(err)
	}
	var back Job
	if err := back.UnmarshalJSON(raw); err != nil {
		t.Fatal(err)
	}
	if !back.GetNightQueue() || !back.GetNightPaused() {
		t.Fatalf("flags lost in persistence: %s", raw)
	}
}
