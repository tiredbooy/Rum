package download

import (
	"context"
)

func DownloadWorker(ctx context.Context, job *Job) {
	err := DownloadSingleFile(ctx, *Opt, job.URL)

	mu.Lock()
	if err == context.Canceled {
		job.Status = "paused"
	} else if err == nil {
		job.Status = "completed"
		resultChan <- DownloadResult{URL: job.URL, Success: true, Error: nil}
	} else {
		job.Status = "error"
		job.Error = err
		resultChan <- DownloadResult{URL: job.URL, Success: false, Error: err}
	}
	mu.Unlock()

}
