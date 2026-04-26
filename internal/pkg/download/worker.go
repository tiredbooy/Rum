package download

import (
	"context"
)

type ProgressFunc func(downloaded, total int64)

func DownloadWorker(ctx context.Context, opt *Options, job *Job, progressFn ProgressFunc) {
	err := DownloadSingleFile(ctx, *Opt, job, progressFn)

	mu.Lock()
	defer mu.Unlock()

	if ctx.Err() == context.Canceled {
		job.Status = "paused"
		return
	}

	if err == nil {
		job.Status = "completed"
		resultChan <- DownloadResult{URL: job.URL, Success: true, Error: nil}
	} else if err == context.Canceled {
		job.Status = "paused"
	} else {
		job.Status = "error"
		job.Error = err
		resultChan <- DownloadResult{URL: job.URL, Success: false, Error: err}
	}
}
