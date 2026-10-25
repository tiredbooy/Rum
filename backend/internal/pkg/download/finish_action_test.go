package download

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/tiredbooy/Rum/backend/internal/pkg/config"
)

type finishRig struct {
	f                  *finishAction
	action             atomic.Value // string
	busy               atomic.Bool
	quits, sleeps, off atomic.Int32
}

func newFinishRig(delay time.Duration, action string) *finishRig {
	r := &finishRig{}
	r.action.Store(action)
	r.f = newFinishAction(delay,
		func() string { return r.action.Load().(string) },
		func() bool { return r.busy.Load() },
		func() { r.quits.Add(1) },
		func() { r.sleeps.Add(1) },
		func() { r.off.Add(1) },
	)
	return r
}

func waitCount(t *testing.T, c *atomic.Int32, want int32) {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if c.Load() == want {
			return
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatalf("count = %d, want %d", c.Load(), want)
}

func TestFinishActionDoesNothingWhileBusy(t *testing.T) {
	r := newFinishRig(10*time.Millisecond, "shutdown")
	r.busy.Store(true)
	r.f.noteCompleted()
	r.f.evaluate()
	time.Sleep(40 * time.Millisecond)
	if r.off.Load() != 0 {
		t.Fatal("fired while queue busy")
	}
	if _, ok := r.f.snapshot(); ok {
		t.Fatal("countdown started while queue busy")
	}
	r.busy.Store(false)
	r.f.evaluate()
	waitCount(t, &r.off, 1)
}

func TestFinishActionNeedsACompletion(t *testing.T) {
	r := newFinishRig(5*time.Millisecond, "sleep")
	r.f.evaluate() // idle, but nothing was ever downloaded
	time.Sleep(30 * time.Millisecond)
	if r.sleeps.Load() != 0 {
		t.Fatal("fired with no completed download")
	}
}

func TestFinishActionCloseIsImmediateAndOnce(t *testing.T) {
	r := newFinishRig(time.Hour, "close")
	r.f.noteCompleted()
	r.f.evaluate()
	r.f.evaluate()
	if r.quits.Load() != 1 {
		t.Fatalf("quit count = %d, want 1", r.quits.Load())
	}
}

func TestFinishActionNoneNeverFiresAndDisarms(t *testing.T) {
	r := newFinishRig(5*time.Millisecond, "none")
	r.f.noteCompleted()
	r.f.evaluate()
	r.action.Store("sleep")
	r.f.evaluate() // stale completion must not carry over once the user opts in
	time.Sleep(30 * time.Millisecond)
	if r.sleeps.Load() != 0 {
		t.Fatal("fired from a stale completion")
	}
}

func TestFinishActionCountdownCancel(t *testing.T) {
	r := newFinishRig(60*time.Millisecond, "sleep")
	r.f.noteCompleted()
	r.f.evaluate()
	p, ok := r.f.snapshot()
	if !ok || p.Action != "sleep" || p.SecondsLeft < 0 {
		t.Fatalf("snapshot = %+v ok=%v", p, ok)
	}
	if !r.f.cancel() {
		t.Fatal("cancel reported nothing pending")
	}
	time.Sleep(120 * time.Millisecond)
	if r.sleeps.Load() != 0 {
		t.Fatal("slept after cancel")
	}
	if _, ok := r.f.snapshot(); ok {
		t.Fatal("still pending after cancel")
	}
	if r.f.cancel() {
		t.Fatal("second cancel should report false")
	}
}

func TestFinishActionNewWorkCancelsCountdown(t *testing.T) {
	r := newFinishRig(40*time.Millisecond, "shutdown")
	r.f.noteCompleted()
	r.f.evaluate()
	r.f.activity()
	time.Sleep(100 * time.Millisecond)
	if r.off.Load() != 0 {
		t.Fatal("shut down although new work started")
	}
}

func TestFinishActionRechecksAtFireTime(t *testing.T) {
	r := newFinishRig(40*time.Millisecond, "shutdown")
	r.f.noteCompleted()
	r.f.evaluate()
	r.busy.Store(true) // a job slipped in without going through activity()
	time.Sleep(100 * time.Millisecond)
	if r.off.Load() != 0 {
		t.Fatal("shut down while a job was running")
	}

	r2 := newFinishRig(40*time.Millisecond, "shutdown")
	r2.f.noteCompleted()
	r2.f.evaluate()
	r2.action.Store("none") // user changed their mind during the countdown
	time.Sleep(100 * time.Millisecond)
	if r2.off.Load() != 0 {
		t.Fatal("shut down after the action was switched to none")
	}
}

func TestFinishActionSingleCountdown(t *testing.T) {
	r := newFinishRig(30*time.Millisecond, "sleep")
	r.f.noteCompleted()
	r.f.evaluate()
	r.f.noteCompleted()
	r.f.evaluate()
	waitCount(t, &r.sleeps, 1)
	time.Sleep(60 * time.Millisecond)
	if r.sleeps.Load() != 1 {
		t.Fatalf("slept %d times", r.sleeps.Load())
	}
}

// --- manager integration: one job finishing must not trigger the action ---

func setPostDownloadAction(t *testing.T, action string) {
	t.Helper()
	var s config.Setting
	_ = s.LoadSettingMetadata()
	s.PostDownload.Action = action
	if err := s.Save(); err != nil {
		t.Fatal(err)
	}
}

func TestManagerFinishActionWaitsForWholeQueue(t *testing.T) {
	release := make(chan struct{})
	var once sync.Once
	open := func() { once.Do(func() { close(release) }) }
	defer open()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/slow.bin" {
			select {
			case <-release:
			case <-r.Context().Done():
				return
			}
		}
		w.Write([]byte("\x00hello")) // NUL: not text/plain, so not read as a block page
	}))
	defer srv.Close()

	m, _ := newManagerForTest(t, 2)
	defer m.Shutdown()
	setPostDownloadAction(t, "close")

	var quits atomic.Int32
	prev := quitFunc
	SetQuitFunc(func() { quits.Add(1) })
	defer SetQuitFunc(prev)

	for _, name := range []string{"fast.bin", "slow.bin"} {
		m.mu.Lock()
		m.jobs[name] = &Job{ID: name, URL: srv.URL + "/" + name, FileName: name,
			TotalSize: -1, Status: StatusPending, Priority: "normal"}
		m.mu.Unlock()
		if err := m.StartJob(nil, name); err != nil {
			t.Fatal(err)
		}
	}
	fast, _ := m.GetJob("fast.bin")
	waitStatus(t, fast, StatusCompleted)
	time.Sleep(150 * time.Millisecond)
	if quits.Load() != 0 {
		t.Fatal("per-job completion fired the finish action while another job was running")
	}

	open()
	slow, _ := m.GetJob("slow.bin")
	waitStatus(t, slow, StatusCompleted)
	waitCount(t, &quits, 1)
}

var errPermanentForTest = errors.New("checksum mismatch")

func TestQueueBusyStates(t *testing.T) {
	m, _ := newManagerForTest(t, 1)
	defer m.Shutdown()
	setPostDownloadAction(t, "none")

	add := func(id, status string, err error) {
		m.mu.Lock()
		m.jobs[id] = &Job{ID: id, Status: status, Error: err}
		m.mu.Unlock()
	}
	del := func(id string) { m.mu.Lock(); delete(m.jobs, id); m.mu.Unlock() }

	add("done", StatusCompleted, nil)
	add("hand-paused", StatusPaused, nil)
	if m.queueBusy() {
		t.Fatal("completed + user-paused jobs must not count as busy")
	}
	for _, st := range []string{StatusRunning, StatusPending} {
		add("x", st, nil)
		if !m.queueBusy() {
			t.Fatalf("%s job must be busy", st)
		}
		del("x")
	}
	add("full", StatusError, &DiskFullError{NeededBytes: 10, FreeBytes: 1, Path: "/"})
	if !m.queueBusy() {
		t.Fatal("job waiting for disk space must be busy")
	}
	del("full")
	add("broken", StatusError, errPermanentForTest)
	if m.queueBusy() {
		t.Fatal("permanently failed job must not be busy")
	}
	del("broken")

	// A network failure that auto-resume-on-reconnect will retry is still queued.
	add("flaky", StatusError, errStalled)
	var s config.Setting
	_ = s.LoadSettingMetadata()
	s.AutoResumeOnReconnect = true
	_ = s.Save()
	if !m.queueBusy() {
		t.Fatal("reconnecting job must be busy")
	}
	s.AutoResumeOnReconnect = false
	_ = s.Save()
	if m.queueBusy() {
		t.Fatal("with auto-resume off a failed job is final")
	}
}
