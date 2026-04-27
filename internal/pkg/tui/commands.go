package tui

import (
	"context"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"swiftget.com/internal/pkg/download"
)

var progressThrottle = newThrottler()

var sem chan struct{}

func InitWorkerPool(parallel int) {
	if parallel < 1 {
		parallel = 1
	}
	sem = make(chan struct{}, parallel)
}

func startDownloadCmd(job *download.Job, opt *download.Options, p *tea.Program) tea.Cmd {
	return func() tea.Msg {
		go func() {
			ctx, cancel := context.WithCancel(context.Background())
			job.CancelFunc = cancel

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

				if downloaded == total || progressThrottle.shouldSend(job.ID, 200*time.Millisecond) {
					p.Send(progressMsg{jobID: job.ID, downloaded: downloaded, total: total})
				}
			})

			// Send final state
			p.Send(jobDoneMsg{jobID: job.ID, err: err})
		}()
		return nil
	}
}