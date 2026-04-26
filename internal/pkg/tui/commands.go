package tui

import (
	"context"
	"fmt"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"swiftget.com/internal/pkg/download"
)

var progressThrottle = newThrottler()

// Worker pool for parallelism control
var sem chan struct{}

func InitWorkerPool(parallel int) {
	if parallel < 1 {
		parallel = 1
	}
	sem = make(chan struct{}, parallel)
}

func runJob(job *download.Job, p *tea.Program, opt *download.Options) {
	fmt.Printf("DEBUG: runJob START for %s, status=%s, downloaded=%d\n", job.ID[:8], job.Status, job.Downloaded)

	sem <- struct{}{}
	defer func() { <-sem }()
	fmt.Printf("DEBUG: runJob acquired semaphore for %s\n", job.ID[:8])

	ctx, cancel := context.WithCancel(context.Background())
	job.CancelFunc = cancel
	job.Status = "running"

	var (
		lastDownloaded int64
		lastTime       time.Time
	)

	download.DownloadWorker(ctx, opt, job, func(downloaded, total int64) {
		now := time.Now()
		if !lastTime.IsZero() {
			elapsed := now.Sub(lastTime).Seconds()
			if elapsed > 0 {
				bytesDelta := downloaded - lastDownloaded
				job.Speed = float64(bytesDelta) / elapsed
				if job.Speed > 0 && job.TotalSize > 0 {
					remainingBytes := job.TotalSize - downloaded
					job.RemainingTime = time.Duration(float64(remainingBytes)/job.Speed) * time.Second
				}
			}
		}
		lastDownloaded = downloaded
		lastTime = now

		// Throttle: send at most every 200ms
		if progressThrottle.shouldSend(job.ID, 200*time.Millisecond) {
			p.Send(progressMsg{jobID: job.ID, downloaded: downloaded, total: total})
		}
		// Also send when finished
		if downloaded == total {
			p.Send(progressMsg{jobID: job.ID, downloaded: downloaded, total: total})
		}
	})

	fmt.Printf("DEBUG: runJob FINISHED for %s\n", job.ID[:8])
	p.Send(jobDoneMsg{jobID: job.ID, err: nil})
}
