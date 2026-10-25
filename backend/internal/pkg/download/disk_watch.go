package download

import (
	"context"
	"log"
	"path/filepath"
	"sync"
	"time"
)

// diskWatchInterval is how often downloads waiting for disk space are checked.
// A statfs per volume is cheap, so this can be tight enough to feel automatic
// after the user deletes something.
const diskWatchInterval = 12 * time.Second

// DiskWatcher resumes downloads that stopped because a volume was full, once
// that volume has room again. Those jobs sit in the error state with a
// *DiskFullError (persisted, so they survive a restart) and are deliberately
// invisible to the reconnect controller, which only retries network failures.
//
// To avoid a start/fail loop it resumes a job only when free space has GROWN
// since the failure and now covers the remaining bytes plus the safety margin.
type DiskWatcher struct {
	manager  *JobManager
	interval time.Duration

	cancel context.CancelFunc
	wg     sync.WaitGroup
	once   sync.Once
}

// NewDiskWatcher builds a watcher bound to manager.
func NewDiskWatcher(manager *JobManager) *DiskWatcher {
	return &DiskWatcher{manager: manager, interval: diskWatchInterval}
}

// Start launches the polling goroutine.
func (w *DiskWatcher) Start(ctx context.Context) {
	runCtx, cancel := context.WithCancel(ctx)
	w.cancel = cancel
	w.wg.Add(1)
	go func() {
		defer w.wg.Done()
		ticker := time.NewTicker(w.interval)
		defer ticker.Stop()
		for {
			select {
			case <-runCtx.Done():
				return
			case <-ticker.C:
				w.tick(runCtx)
			}
		}
	}()
}

// Stop cancels the goroutine and waits for it. Idempotent.
func (w *DiskWatcher) Stop() {
	w.once.Do(func() {
		if w.cancel != nil {
			w.cancel()
		}
		w.wg.Wait()
	})
}

// tick resumes every waiting job whose volume now has enough free space. Free
// space is read once per distinct directory.
func (w *DiskWatcher) tick(ctx context.Context) {
	if w.manager == nil {
		return
	}
	free := map[string]int64{}
	for _, job := range w.manager.GetAllJobs() {
		if ctx.Err() != nil {
			return
		}
		if job.GetStatus() != StatusError {
			continue
		}
		df, ok := AsDiskFull(job.GetError())
		if !ok {
			continue
		}
		dir := df.Path
		if dir == "" {
			dir = filepath.Dir(job.GetOutputPath())
		}
		avail, seen := free[dir]
		if !seen {
			got, known := diskFreeFn(dir)
			if !known {
				continue
			}
			avail = got
			free[dir] = got
		}
		if avail <= df.FreeBytes || avail < requiredWithMargin(df.NeededBytes) {
			continue
		}
		if w.manager.NightHolds(job, time.Now()) {
			continue // night job: retried inside its window, not before
		}
		if err := w.manager.RetryJob(ctx, job.ID); err != nil {
			log.Printf("auto-resume on free space: %s: %v", job.ID, err)
		}
	}
}
