// Package scheduler provides a global concurrency scheduler backed by a
// priority queue. It gates how many jobs may run concurrently while ordering
// pending work by priority (FIFO within a level).
//
// Concurrency design (per go-concurrency rules):
//   - All shared state (running set, queued set, max, closed flag) is guarded
//     by a single sync.Mutex; a sync.Cond layered on that mutex coordinates
//     waiters so the limit can be resized at runtime (a fixed-size channel
//     cannot). Next blocks on the cond until a slot frees, work arrives, the
//     context cancels, or the scheduler closes — there is no busy-wait.
//   - The underlying queue.PriorityQueue stays the source of truth for ordering
//     but is only ever Pushed/TryPopped under the scheduler mutex, so the two
//     stay consistent. We never call the queue's blocking Pop; Next does its own
//     waiting via the cond so it can also wake on Done/SetMax/Close.
//   - The only goroutine ever spawned is a short-lived per-Next ctx watcher that
//     is always reaped, so the scheduler cannot leak goroutines.
package scheduler

import (
	"context"
	"sync"

	"github.com/tiredbooy/Rum/backend/internal/pkg/queue"
)

// Scheduler enforces a global concurrency limit over a priority-ordered set of
// job ids. Submit enqueues, Next hands out the next runnable id (blocking until
// a slot is free and work is available), and Done frees a slot.
type Scheduler struct {
	mu      sync.Mutex
	cond    *sync.Cond
	pq      *queue.PriorityQueue
	max     int
	running map[string]struct{}
	queued  map[string]struct{}
	closed  bool
}

// New returns a Scheduler permitting at most maxConcurrent running jobs.
// A maxConcurrent < 1 is clamped to 1.
func New(maxConcurrent int) *Scheduler {
	if maxConcurrent < 1 {
		maxConcurrent = 1
	}
	s := &Scheduler{
		pq:      queue.NewPriorityQueue(0), // unbounded
		max:     maxConcurrent,
		running: make(map[string]struct{}),
		queued:  make(map[string]struct{}),
	}
	s.cond = sync.NewCond(&s.mu)
	return s
}

// Submit enqueues job id at priority p. Submitting an id that is already queued
// or already running is a no-op (dedupe). Submit after Close is a no-op.
func (s *Scheduler) Submit(id string, p queue.Priority) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return
	}
	if _, ok := s.queued[id]; ok {
		return
	}
	if _, ok := s.running[id]; ok {
		return
	}
	// TryPush on an unbounded, open queue never blocks and never fails.
	if ok, err := s.pq.TryPush(id, p); err != nil || !ok {
		return
	}
	s.queued[id] = struct{}{}
	s.cond.Broadcast()
}

// Next blocks until a concurrency slot is free AND a queued item is available,
// then marks that item running and returns its id. It returns ("", false) when
// ctx is cancelled or the scheduler is closed (and drained of grantable work).
func (s *Scheduler) Next(ctx context.Context) (string, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Watch ctx with a short-lived goroutine that broadcasts on cancellation so
	// the cond.Wait below wakes. The goroutine is always reaped via stop.
	var stop chan struct{}
	if ctx != nil {
		if err := ctx.Err(); err != nil {
			return "", false
		}
		stop = make(chan struct{})
		var once sync.Once
		defer once.Do(func() { close(stop) })
		go func() {
			select {
			case <-ctx.Done():
				s.mu.Lock()
				s.cond.Broadcast()
				s.mu.Unlock()
			case <-stop:
			}
		}()
	}

	for {
		if s.closed {
			return "", false
		}
		if ctx != nil && ctx.Err() != nil {
			return "", false
		}
		if len(s.running) < s.max {
			if v, ok, _ := s.pq.TryPop(); ok {
				id := v.(string)
				delete(s.queued, id)
				s.running[id] = struct{}{}
				return id, true
			}
		}
		// Either no slot or no work: wait to be woken by Submit/Done/SetMax/ctx/Close.
		s.cond.Wait()
	}
}

// Done marks a running job finished, freeing its slot and waking a waiter.
// Done for an id that is not running is a no-op.
func (s *Scheduler) Done(id string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.running[id]; !ok {
		return
	}
	delete(s.running, id)
	s.cond.Broadcast()
}

// SetMax adjusts the concurrency limit at runtime. n is clamped to >= 1. If the
// limit is raised, waiters are woken so freshly available slots are filled.
func (s *Scheduler) SetMax(n int) {
	if n < 1 {
		n = 1
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.max = n
	s.cond.Broadcast()
}

// Close shuts the scheduler down: no further Submit/Next succeed, and every
// blocked Next is woken so it can return. Close is idempotent.
func (s *Scheduler) Close() {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return
	}
	s.closed = true
	s.pq.Close()
	s.cond.Broadcast()
}

// Running returns the number of jobs currently marked running.
func (s *Scheduler) Running() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return len(s.running)
}

// Queued returns the number of jobs waiting to be granted a slot.
func (s *Scheduler) Queued() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return len(s.queued)
}

// Max returns the current concurrency limit.
func (s *Scheduler) Max() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.max
}

// ParsePriority maps a human string to a queue.Priority. "low"/"high" map to
// the matching levels; anything else (including "normal" or "") returns
// PriorityNormal.
func ParsePriority(s string) queue.Priority {
	switch s {
	case "low", "Low", "LOW":
		return queue.PriorityLow
	case "high", "High", "HIGH":
		return queue.PriorityHigh
	default:
		return queue.PriorityNormal
	}
}
