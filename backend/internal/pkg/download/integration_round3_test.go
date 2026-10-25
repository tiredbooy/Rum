package download

import (
	"context"
	"testing"
	"time"

	"github.com/tiredbooy/Rum/backend/internal/pkg/config"
	"github.com/tiredbooy/Rum/backend/internal/pkg/power"
)

// closedWindow returns a night plan whose window is several hours away from now.
func closedWindow() config.NightPlan {
	now := time.Now()
	return nightPlan(now.Add(3*time.Hour).Format("15:04"), now.Add(5*time.Hour).Format("15:04"))
}

// The finish action must not power off / quit while night jobs wait for their
// window, and the sleep inhibitor must not be held for them. A job the user
// paused is not queued work, and a soft-deleted job is invisible to both.
func TestQueueBusyCountsNightWaitingJobsButNotUserPausedOrHidden(t *testing.T) {
	m := newNightManager(t, closedWindow())
	defer m.Shutdown()

	if m.queueBusy() {
		t.Fatal("empty queue reported busy")
	}
	user := addPlainJob(m, "u", StatusPaused)
	if m.queueBusy() {
		t.Fatal("user-paused job counted as queued")
	}
	user.SetStatus(StatusCompleted)

	n := addNightJob(m, "n", StatusPaused)
	n.SetNightPaused(true)
	if !m.queueBusy() {
		t.Fatal("night job parked by the schedule must keep the queue busy")
	}

	if _, err := m.SoftDeleteJob("n", false); err != nil {
		t.Fatal(err)
	}
	if m.queueBusy() {
		t.Fatal("soft-deleted night job still counted as queued")
	}
}

func TestNightWindowCloseReleasesInhibitorButQueueStaysBusy(t *testing.T) {
	srv := newEndlessServer(t)
	defer srv.Close()
	m, _ := newManagerForTest(t, 1)
	defer m.Shutdown()
	setNightPlan(m, nightPlan("02:00", "07:00"))

	fb := &countingBackend{}
	in := power.NewInhibitor(fb)
	in.SetEnabled(true)
	m.SetSleepInhibitor(in)

	m.mu.Lock()
	j := &Job{ID: "n", URL: srv.URL + "/n", FileName: "n.bin", TotalSize: -1, Status: StatusPending, Priority: "normal"}
	j.SetNightQueue(true)
	m.jobs["n"] = j
	m.mu.Unlock()
	if err := m.StartJob(nil, "n"); err != nil {
		t.Fatal(err)
	}
	waitHeld(t, fb, 1)

	c := NewNightController(m)
	c.tick(context.Background(), nightIn)  // window open
	c.tick(context.Background(), nightOut) // window closes: job parked
	waitHeld(t, fb, 0)
	if !j.GetNightPaused() {
		t.Fatal("job not flagged for the next window")
	}
	if !m.queueBusy() {
		t.Fatal("finish action would fire while a night job waits for its window")
	}
}

// Undoing a delete of a queued night job outside its window must not start it.
func TestRestoreOfNightJobOutsideWindowStaysParked(t *testing.T) {
	m := newNightManager(t, closedWindow())
	defer m.Shutdown()
	j := addNightJob(m, "n", StatusPending)

	if _, err := m.SoftDeleteJob("n", false); err != nil {
		t.Fatal(err)
	}
	if err := m.RestoreJob("n"); err != nil {
		t.Fatal(err)
	}
	if got := j.GetStatus(); got == StatusRunning || got == StatusPending {
		t.Fatalf("restored night job status = %s outside the window", got)
	}
	if !j.GetNightPaused() {
		t.Fatal("restored night job not flagged for the next window")
	}
}
