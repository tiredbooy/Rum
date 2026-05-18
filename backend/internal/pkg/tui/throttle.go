package tui

import (
	"sync"
	"time"
)

type throttler struct {
	mu       sync.Mutex
	lastSent map[string]time.Time
}

func newThrottler() *throttler {
	return &throttler{lastSent: make(map[string]time.Time)}
}

func (t *throttler) shouldSend(jobID string, minInterval time.Duration) bool {
	t.mu.Lock()
	defer t.mu.Unlock()
	now := time.Now()
	last, ok := t.lastSent[jobID]
	if !ok || now.Sub(last) >= minInterval {
		t.lastSent[jobID] = now
		return true
	}
	return false
}
