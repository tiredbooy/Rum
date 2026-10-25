package download

import (
	"log"
	"sync"
	"time"

	"github.com/tiredbooy/Rum/backend/internal/pkg/config"
	"github.com/tiredbooy/Rum/backend/internal/pkg/utils"
)

// finishCountdown is how long the UI gets to cancel a pending sleep/shutdown.
const finishCountdown = 60 * time.Second

// FinishPending describes a countdown that will sleep or shut down the machine
// unless it is cancelled.
type FinishPending struct {
	Action      string `json:"action"` // "sleep" | "shutdown"
	DeadlineMs  int64  `json:"deadline_ms"`
	SecondsLeft int    `json:"seconds_left"`
}

// finishAction runs the global "when all downloads finish" action. It fires only
// when the whole queue is idle (never because one job of several finished) and
// only after at least one download actually completed. Quit is immediate;
// sleep/shutdown first run a cancelable countdown that is re-validated when it
// expires.
type finishAction struct {
	delay     time.Duration
	getAction func() string
	busy      func() bool
	quit      func()
	sleep     func()
	shutdown  func()

	mu       sync.Mutex
	armed    bool // a download completed since the last firing/disarm
	pending  *FinishPending
	timer    *time.Timer
	deadline time.Time
}

func newFinishAction(delay time.Duration, getAction func() string, busy func() bool, quit, sleep, shutdown func()) *finishAction {
	return &finishAction{delay: delay, getAction: getAction, busy: busy, quit: quit, sleep: sleep, shutdown: shutdown}
}

// noteCompleted arms the action: something was downloaded.
func (f *finishAction) noteCompleted() {
	f.mu.Lock()
	f.armed = true
	f.mu.Unlock()
}

// evaluate is called when a download ends on its own (completed or failed).
func (f *finishAction) evaluate() {
	f.mu.Lock()
	if !f.armed || f.pending != nil || f.busy() {
		f.mu.Unlock()
		return
	}
	action := f.getAction()
	if action != "close" && action != "sleep" && action != "shutdown" {
		f.armed = false // nothing wanted; don't let this completion fire a later opt-in
		f.mu.Unlock()
		return
	}
	f.armed = false
	if action == "close" {
		f.mu.Unlock()
		f.quit()
		return
	}
	f.deadline = time.Now().Add(f.delay)
	p := &FinishPending{Action: action, DeadlineMs: f.deadline.UnixMilli()}
	f.pending = p
	f.timer = time.AfterFunc(f.delay, func() { f.fire(p) })
	f.mu.Unlock()
}

func (f *finishAction) fire(p *FinishPending) {
	f.mu.Lock()
	if f.pending != p { // cancelled or superseded
		f.mu.Unlock()
		return
	}
	f.pending, f.timer = nil, nil
	stillOK := !f.busy() && f.getAction() == p.Action
	f.mu.Unlock()
	if !stillOK {
		return
	}
	switch p.Action {
	case "sleep":
		f.sleep()
	case "shutdown":
		f.shutdown()
	}
}

// cancel aborts a pending countdown; it reports whether one was pending.
func (f *finishAction) cancel() bool {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.pending == nil {
		return false
	}
	f.timer.Stop()
	f.pending, f.timer = nil, nil
	return true
}

// activity is called when a download starts: new work voids a pending countdown.
func (f *finishAction) activity() { f.cancel() }

func (f *finishAction) snapshot() (FinishPending, bool) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.pending == nil {
		return FinishPending{}, false
	}
	p := *f.pending
	if left := time.Until(f.deadline); left > 0 {
		p.SecondsLeft = int((left + time.Second - 1) / time.Second)
	}
	return p, true
}

// PendingFinishAction returns the running sleep/shutdown countdown, if any.
func (m *JobManager) PendingFinishAction() (FinishPending, bool) { return m.fin.snapshot() }

// CancelFinishAction aborts the countdown; false means there was none.
func (m *JobManager) CancelFinishAction() bool { return m.fin.cancel() }

func (m *JobManager) newFinishAction() *finishAction {
	return newFinishAction(finishCountdown,
		func() string {
			var s config.Setting
			if err := s.LoadSettingMetadata(); err != nil {
				return "none"
			}
			return s.PostDownload.Action
		},
		m.queueBusy,
		func() {
			if quitFunc != nil {
				quitFunc()
			}
		},
		func() {
			if err := utils.SleepPC(); err != nil {
				log.Printf("Sleep failed: %v", err)
			}
		},
		func() {
			if err := utils.ShutdownPC(); err != nil {
				log.Printf("Shutdown failed: %v", err)
			}
		},
	)
}

// queueBusy reports whether any download is still to come: running, pending
// (incl. scheduled), waiting for free disk space, or failed with a network error
// that auto-resume-on-reconnect will retry. User-paused jobs are not queued.
func (m *JobManager) queueBusy() bool {
	var s config.Setting
	_ = s.LoadSettingMetadata()
	for _, j := range m.GetAllJobs() {
		switch j.GetStatus() {
		case StatusRunning, StatusPending:
			return true
		case StatusError:
			if _, full := AsDiskFull(j.GetError()); full {
				return true
			}
			if s.AutoResumeOnReconnect && jobAutoResumable(j) {
				return true
			}
		}
	}
	return false
}
