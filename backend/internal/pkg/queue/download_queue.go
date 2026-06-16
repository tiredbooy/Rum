// Package queue provides concurrency primitives used by the download engine.
//
// The historical Download struct is preserved for backward compatibility. The
// new PriorityQueue is a self-contained, thread-safe, context-aware blocking
// priority queue (FIFO within a priority level) that the engine can adopt to
// schedule downloads without leaking goroutines.
package queue

import (
	"container/heap"
	"context"
	"errors"
	"sync"
	"time"
)

// Download is the legacy job descriptor. Kept for backward compatibility.
type Download struct {
	ID        string     `bson:"_id"`
	URL       string     `bson:"url"`
	Status    string     `bson:"status"`
	StartTime time.Time  `bson:"start_time"`
	EndTime   *time.Time `bson:"end_time"`
}

// Priority levels for queued items. Higher numeric value = scheduled first.
type Priority int

const (
	PriorityLow    Priority = 0
	PriorityNormal Priority = 10
	PriorityHigh   Priority = 20
)

// ErrQueueClosed is returned by Push/Pop once the queue has been closed.
var ErrQueueClosed = errors.New("queue: closed")

// Item is a unit of work stored in the PriorityQueue. Value is opaque to the
// queue; the engine typically stores a job ID or *Download here.
type Item struct {
	Value    any
	Priority Priority

	// seq preserves FIFO ordering for items of equal priority. Set internally.
	seq uint64
	// index is maintained by container/heap. -1 once removed.
	index int
}

// itemHeap is the internal max-heap ordered by (Priority desc, seq asc).
// It is NOT safe for concurrent use on its own; PriorityQueue guards it.
type itemHeap []*Item

func (h itemHeap) Len() int { return len(h) }

func (h itemHeap) Less(i, j int) bool {
	if h[i].Priority != h[j].Priority {
		return h[i].Priority > h[j].Priority // higher priority first
	}
	return h[i].seq < h[j].seq // older insertion first (FIFO)
}

func (h itemHeap) Swap(i, j int) {
	h[i], h[j] = h[j], h[i]
	h[i].index = i
	h[j].index = j
}

func (h *itemHeap) Push(x any) {
	it := x.(*Item)
	it.index = len(*h)
	*h = append(*h, it)
}

func (h *itemHeap) Pop() any {
	old := *h
	n := len(old)
	it := old[n-1]
	old[n-1] = nil // avoid memory leak
	it.index = -1
	*h = old[:n-1]
	return it
}

// PriorityQueue is a thread-safe, context-aware blocking priority queue.
//
// Concurrency design (per go-concurrency rules):
//   - All shared state (heap, closed flag, seq counter) is guarded by mu.
//   - Blocking Pop waits on a condition variable; Close broadcasts so every
//     waiter has a clear exit. Pop also honours ctx.Done() via a per-call
//     watcher goroutine that is always reaped (no leak).
//   - There are no long-lived internal goroutines; the queue owns no worker
//     pool of its own, so it cannot leak goroutines.
type PriorityQueue struct {
	mu     sync.Mutex
	cond   *sync.Cond
	heap   itemHeap
	seq    uint64
	closed bool

	// cap is the maximum number of items; 0 means unbounded.
	cap int
}

// NewPriorityQueue returns an empty queue. A capacity of 0 means unbounded.
// Negative capacities are treated as 0 (unbounded).
func NewPriorityQueue(capacity int) *PriorityQueue {
	if capacity < 0 {
		capacity = 0
	}
	pq := &PriorityQueue{cap: capacity}
	pq.cond = sync.NewCond(&pq.mu)
	heap.Init(&pq.heap)
	return pq
}

// Len returns the current number of queued items.
func (pq *PriorityQueue) Len() int {
	pq.mu.Lock()
	defer pq.mu.Unlock()
	return pq.heap.Len()
}

// Push enqueues value at the given priority. It returns ErrQueueClosed if the
// queue is closed. If the queue is bounded and full, Push blocks until space
// is available, the context is cancelled, or the queue is closed.
func (pq *PriorityQueue) Push(ctx context.Context, value any, priority Priority) error {
	pq.mu.Lock()
	defer pq.mu.Unlock()

	for {
		if pq.closed {
			return ErrQueueClosed
		}
		if pq.cap == 0 || pq.heap.Len() < pq.cap {
			break
		}
		// Bounded and full: wait for a slot, watching ctx.
		if err := pq.wait(ctx); err != nil {
			return err
		}
	}

	pq.seq++
	heap.Push(&pq.heap, &Item{Value: value, Priority: priority, seq: pq.seq})
	pq.cond.Broadcast()
	return nil
}

// TryPush enqueues without blocking. It returns false if the queue is full.
// It returns ErrQueueClosed if the queue is closed.
func (pq *PriorityQueue) TryPush(value any, priority Priority) (bool, error) {
	pq.mu.Lock()
	defer pq.mu.Unlock()
	if pq.closed {
		return false, ErrQueueClosed
	}
	if pq.cap != 0 && pq.heap.Len() >= pq.cap {
		return false, nil
	}
	pq.seq++
	heap.Push(&pq.heap, &Item{Value: value, Priority: priority, seq: pq.seq})
	pq.cond.Broadcast()
	return true, nil
}

// Pop removes and returns the highest-priority item, blocking until one is
// available, the context is cancelled, or the queue is closed and drained.
// When the queue is closed and empty it returns ErrQueueClosed.
func (pq *PriorityQueue) Pop(ctx context.Context) (any, error) {
	pq.mu.Lock()
	defer pq.mu.Unlock()

	for pq.heap.Len() == 0 {
		if pq.closed {
			return nil, ErrQueueClosed
		}
		if err := pq.wait(ctx); err != nil {
			return nil, err
		}
	}

	it := heap.Pop(&pq.heap).(*Item)
	// A slot freed up; wake any blocked Push waiters.
	pq.cond.Broadcast()
	return it.Value, nil
}

// TryPop removes and returns the highest-priority item without blocking.
// ok is false when the queue is empty. It returns ErrQueueClosed only when the
// queue is closed AND empty.
func (pq *PriorityQueue) TryPop() (value any, ok bool, err error) {
	pq.mu.Lock()
	defer pq.mu.Unlock()
	if pq.heap.Len() == 0 {
		if pq.closed {
			return nil, false, ErrQueueClosed
		}
		return nil, false, nil
	}
	it := heap.Pop(&pq.heap).(*Item)
	pq.cond.Broadcast()
	return it.Value, true, nil
}

// Close marks the queue closed and wakes all blocked goroutines so they exit.
// Items already queued can still be drained with Pop/TryPop. Close is
// idempotent.
func (pq *PriorityQueue) Close() {
	pq.mu.Lock()
	defer pq.mu.Unlock()
	if pq.closed {
		return
	}
	pq.closed = true
	pq.cond.Broadcast()
}

// Closed reports whether Close has been called.
func (pq *PriorityQueue) Closed() bool {
	pq.mu.Lock()
	defer pq.mu.Unlock()
	return pq.closed
}

// Drain removes and returns all queued items in priority order. Useful for
// shutdown to inspect/persist outstanding work.
func (pq *PriorityQueue) Drain() []any {
	pq.mu.Lock()
	defer pq.mu.Unlock()
	out := make([]any, 0, pq.heap.Len())
	for pq.heap.Len() > 0 {
		it := heap.Pop(&pq.heap).(*Item)
		out = append(out, it.Value)
	}
	pq.cond.Broadcast()
	return out
}

// wait blocks on the condition variable until it is broadcast OR ctx is
// cancelled. It must be called with pq.mu held and returns with it held.
//
// Because sync.Cond cannot select on a context, we spawn a short-lived watcher
// goroutine that broadcasts when ctx is done. The watcher has a guaranteed
// exit: it returns either when ctx fires or when we close the stop channel
// after Wait returns. This keeps the queue leak-free under cancellation.
func (pq *PriorityQueue) wait(ctx context.Context) error {
	if ctx == nil {
		pq.cond.Wait()
		return nil
	}
	// Fast path: already cancelled.
	if err := ctx.Err(); err != nil {
		return err
	}

	stop := make(chan struct{})
	var once sync.Once
	closeStop := func() { once.Do(func() { close(stop) }) }

	go func() {
		select {
		case <-ctx.Done():
			pq.mu.Lock()
			pq.cond.Broadcast()
			pq.mu.Unlock()
		case <-stop:
		}
	}()

	pq.cond.Wait()
	closeStop() // signal the watcher to exit; it will observe stop or ctx.Done
	return ctx.Err()
}
