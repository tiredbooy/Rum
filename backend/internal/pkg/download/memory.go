package download

import (
	"context"
	"runtime/debug"
	"time"
)

// shouldFreeOSMemory reports whether the process is idle enough to force the Go
// runtime to return freed heap pages to the OS. Go releases memory lazily
// (MADV_FREE), so RSS stays high after a big download unless we force it.
func shouldFreeOSMemory(activeCount int) bool {
	return activeCount == 0
}

// StartMemoryController periodically returns idle memory to the OS. It only
// forces a release when no downloads are active, so it never competes with a
// running transfer. It stops when ctx is cancelled.
func StartMemoryController(ctx context.Context, activeCount func() int, interval time.Duration) {
	if interval <= 0 {
		interval = 30 * time.Second
	}
	go func() {
		t := time.NewTicker(interval)
		defer t.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-t.C:
				if shouldFreeOSMemory(activeCount()) {
					debug.FreeOSMemory()
				}
			}
		}
	}()
}
