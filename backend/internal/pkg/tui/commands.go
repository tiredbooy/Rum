package tui

import (
	"context"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/tiredbooy/Rum/backend/internal/pkg/download"
)

var progressThrottle = newThrottler()

var sem chan struct{}

func InitWorkerPool(parallel int) {
	if parallel < 1 {
		parallel = 1
	}
	sem = make(chan struct{}, parallel)
}

// startDownloadCmd launches a single download in its own goroutine. The
// goroutine always has a clear exit:
//   - if the semaphore acquire is cancelled, it reports a Canceled job and
//     returns;
//   - otherwise it runs the download and reports the final state.
//
// The semaphore slot is released via defer on every path that acquired it.
// The job's CancelFunc is stored under the model lock so the Update loop can
// read it without a data race.
func startDownloadCmd(m *model, job *download.Job, opt *download.Options, p *tea.Program) tea.Cmd {
	return func() tea.Msg {
		go func() {
			ctx, cancel := context.WithCancel(context.Background())
			// Store the cancel func under the model lock: Update reads
			// job.CancelFunc while holding m.mu, so the write must too.
			m.mu.Lock()
			job.CancelFunc = cancel
			m.mu.Unlock()

			job.SetStatus("waiting")
			p.Send(progressMsg{jobID: job.ID, downloaded: job.GetDownloaded(), total: job.GetTotalSize()})

			select {
			case sem <- struct{}{}:
				defer func() { <-sem }()
			case <-ctx.Done():
				job.SetStatus("paused")
				p.Send(jobDoneMsg{jobID: job.ID, err: context.Canceled})
				return
			}

			job.SetStatus("running")
			p.Send(progressMsg{jobID: job.ID, downloaded: job.GetDownloaded(), total: job.GetTotalSize()})

			var lastDownloaded int64
			var lastTime time.Time

			err := download.DownloadSingleFile(ctx, *opt, job, func(downloaded, total int64) {
				now := time.Now()
				if !lastTime.IsZero() {
					elapsed := now.Sub(lastTime).Seconds()
					if elapsed > 0 {
						bytesDelta := downloaded - lastDownloaded
						speed := float64(bytesDelta) / elapsed
						job.SetSpeed(speed)
						if speed > 0 && total > 0 {
							remaining := total - downloaded
							job.SetRemainingTime(time.Duration(float64(remaining)/speed) * time.Second)
						} else {
							job.SetRemainingTime(0)
						}
					}
				}
				lastDownloaded = downloaded
				lastTime = now

				// total can be -1 (unknown). Only treat downloaded==total as
				// "finished" when total is actually known (> 0), otherwise we
				// would force a send on every callback for unknown-size files.
				if (total > 0 && downloaded == total) || progressThrottle.shouldSend(job.ID, 200*time.Millisecond) {
					p.Send(progressMsg{jobID: job.ID, downloaded: downloaded, total: total})
				}
			})

			// Send final state
			p.Send(jobDoneMsg{jobID: job.ID, err: err})
		}()
		return nil
	}
}
