package download

import (
	"context"
	"log"
	"path/filepath"
)

// RetryFailedResult counts what a bulk retry did.
type RetryFailedResult struct {
	Retried         int `json:"retried"`
	SkippedDiskFull int `json:"skipped_disk_full"`
	SkippedBlocked  int `json:"skipped_blocked"`
	SkippedExpired  int `json:"skipped_expired"`
}

// retrySkip names why a failed job is left alone by a bulk retry ("" = retry it).
type retrySkip string

const (
	skipNone     retrySkip = ""
	skipDiskFull retrySkip = "disk_full"
	skipBlocked  retrySkip = "blocked_page"
	skipExpired  retrySkip = "link_expired"
)

// retrySkipReason applies the bulk-retry selection rules to one errored job:
// disk-full jobs wait until the volume really has room, block pages need an
// explicit override, and expired links need a new URL first. Everything else is
// retried (the user asked for it; partial data on disk makes it a resume).
// free caches free-space lookups per directory.
func retrySkipReason(job *Job, includeBlocked bool, free map[string]int64) retrySkip {
	code, _ := job.GetErrorCode()
	switch {
	case jobBlocked(job):
		if includeBlocked {
			return skipNone
		}
		return skipBlocked
	case code == ErrCodeLinkExpired:
		return skipExpired
	case code == ErrCodeDiskFull:
		df, ok := AsDiskFull(job.GetError())
		if !ok {
			return skipDiskFull // text-only (reloaded) failure: let the DiskWatcher decide
		}
		dir := df.Path
		if dir == "" {
			dir = filepath.Dir(job.GetOutputPath())
		}
		avail, seen := free[dir]
		if !seen {
			got, known := diskFreeFn(dir)
			if !known {
				return skipDiskFull
			}
			avail = got
			free[dir] = got
		}
		if avail <= df.FreeBytes || avail < requiredWithMargin(df.NeededBytes) {
			return skipDiskFull
		}
	}
	return skipNone
}

// RetryFailed restarts every errored job that is worth retrying now (see
// retrySkipReason). includeBlocked also retries block-page failures, letting
// that run accept the HTML response.
func (m *JobManager) RetryFailed(ctx context.Context, includeBlocked bool) RetryFailedResult {
	var res RetryFailedResult
	free := map[string]int64{}
	for _, job := range m.GetAllJobs() {
		if ctx.Err() != nil {
			break
		}
		if job.GetStatus() != StatusError {
			continue
		}
		switch retrySkipReason(job, includeBlocked, free) {
		case skipDiskFull:
			res.SkippedDiskFull++
			continue
		case skipBlocked:
			res.SkippedBlocked++
			continue
		case skipExpired:
			res.SkippedExpired++
			continue
		}
		if jobBlocked(job) {
			job.SetAllowBlockedPage(true)
		}
		if err := m.RetryJob(ctx, job.ID); err != nil {
			log.Printf("retry failed: %s: %v", job.ID, err)
			continue
		}
		res.Retried++
	}
	return res
}
