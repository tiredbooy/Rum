package scheduler

import (
	"context"
	"fmt"
	"runtime"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/tiredbooy/Rum/backend/internal/pkg/queue"
)

// drainGoroutines waits briefly for spawned goroutines to settle so leak checks
// are not flaky.
func waitGoroutines(target int) bool {
	for i := 0; i < 200; i++ {
		if runtime.NumGoroutine() <= target {
			return true
		}
		time.Sleep(5 * time.Millisecond)
	}
	return false
}

func TestSubmitDedupe(t *testing.T) {
	s := New(2)
	s.Submit("a", queue.PriorityNormal)
	s.Submit("a", queue.PriorityNormal)
	s.Submit("a", queue.PriorityHigh)
	if got := s.Queued(); got != 1 {
		t.Fatalf("expected 1 queued after dedupe, got %d", got)
	}

	// Once running, resubmitting the same id is still a no-op.
	id, ok := s.Next(context.Background())
	if !ok || id != "a" {
		t.Fatalf("expected to get a, got %q ok=%v", id, ok)
	}
	s.Submit("a", queue.PriorityNormal)
	if got := s.Queued(); got != 0 {
		t.Fatalf("expected running id not re-queued, got queued=%d", got)
	}
	if got := s.Running(); got != 1 {
		t.Fatalf("expected 1 running, got %d", got)
	}
}

func TestPriorityOrdering(t *testing.T) {
	s := New(1)
	s.Submit("low", queue.PriorityLow)
	s.Submit("high", queue.PriorityHigh)
	s.Submit("normal", queue.PriorityNormal)

	want := []string{"high", "normal", "low"}
	for _, w := range want {
		id, ok := s.Next(context.Background())
		if !ok {
			t.Fatalf("expected %q, got not ok", w)
		}
		if id != w {
			t.Fatalf("priority order: want %q got %q", w, id)
		}
		s.Done(id)
	}
}

func TestFIFOWithinLevel(t *testing.T) {
	s := New(1)
	for i := 0; i < 5; i++ {
		s.Submit(fmt.Sprintf("j%d", i), queue.PriorityNormal)
	}
	for i := 0; i < 5; i++ {
		id, ok := s.Next(context.Background())
		if !ok {
			t.Fatalf("unexpected not ok at %d", i)
		}
		want := fmt.Sprintf("j%d", i)
		if id != want {
			t.Fatalf("FIFO: want %q got %q", want, id)
		}
		s.Done(id)
	}
}

func TestRespectsMaxConcurrent(t *testing.T) {
	const max = 3
	const jobs = 50
	s := New(max)
	for i := 0; i < jobs; i++ {
		s.Submit(fmt.Sprintf("j%d", i), queue.PriorityNormal)
	}

	var cur, peak int64
	var wg sync.WaitGroup
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	worker := func() {
		defer wg.Done()
		for {
			id, ok := s.Next(ctx)
			if !ok {
				return
			}
			n := atomic.AddInt64(&cur, 1)
			for {
				p := atomic.LoadInt64(&peak)
				if n <= p || atomic.CompareAndSwapInt64(&peak, p, n) {
					break
				}
			}
			if n > max {
				t.Errorf("concurrency exceeded: %d > %d", n, max)
			}
			time.Sleep(time.Millisecond)
			atomic.AddInt64(&cur, -1)
			s.Done(id)
		}
	}

	for i := 0; i < 8; i++ {
		wg.Add(1)
		go worker()
	}

	// Wait until all jobs drained, then close to release workers.
	for s.Running() != 0 || s.Queued() != 0 {
		time.Sleep(time.Millisecond)
	}
	s.Close()
	wg.Wait()

	if p := atomic.LoadInt64(&peak); p > max {
		t.Fatalf("peak concurrency %d > max %d", p, max)
	}
	if p := atomic.LoadInt64(&peak); p == 0 {
		t.Fatalf("no work was processed")
	}
}

func TestSetMaxUp(t *testing.T) {
	s := New(1)
	s.Submit("a", queue.PriorityNormal)
	s.Submit("b", queue.PriorityNormal)

	a, ok := s.Next(context.Background())
	if !ok || a != "a" {
		t.Fatalf("want a, got %q", a)
	}

	// With max=1 and a running, a second Next must block.
	got := make(chan string, 1)
	go func() {
		id, ok := s.Next(context.Background())
		if ok {
			got <- id
		} else {
			got <- "<closed>"
		}
	}()

	select {
	case <-got:
		t.Fatal("Next should block while at max")
	case <-time.After(50 * time.Millisecond):
	}

	// Raising the limit must unblock the waiter without freeing a slot.
	s.SetMax(2)
	select {
	case id := <-got:
		if id != "b" {
			t.Fatalf("want b after SetMax up, got %q", id)
		}
	case <-time.After(time.Second):
		t.Fatal("SetMax up did not unblock waiter")
	}
	if s.Running() != 2 {
		t.Fatalf("expected 2 running, got %d", s.Running())
	}
}

func TestSetMaxDown(t *testing.T) {
	s := New(3)
	s.Submit("a", queue.PriorityNormal)
	s.Submit("b", queue.PriorityNormal)
	a, _ := s.Next(context.Background())
	b, _ := s.Next(context.Background())
	if s.Running() != 2 {
		t.Fatalf("expected 2 running, got %d", s.Running())
	}

	// Lower the limit below current running count.
	s.SetMax(1)
	s.Submit("c", queue.PriorityNormal)

	// No slot should be granted until running drops below new max.
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()
	if id, ok := s.Next(ctx); ok {
		t.Fatalf("Next granted %q while over lowered max", id)
	}

	s.Done(a) // running -> 1, still == max, no grant
	ctx2, cancel2 := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel2()
	if id, ok := s.Next(ctx2); ok {
		t.Fatalf("Next granted %q while at lowered max, got", id)
	}

	s.Done(b) // running -> 0, below max, now grant
	id, ok := s.Next(context.Background())
	if !ok || id != "c" {
		t.Fatalf("want c after draining below max, got %q ok=%v", id, ok)
	}
}

func TestCtxCancelUnblocksNext(t *testing.T) {
	base := runtime.NumGoroutine()
	s := New(1)
	ctx, cancel := context.WithCancel(context.Background())

	done := make(chan bool, 1)
	go func() {
		_, ok := s.Next(ctx) // nothing queued: blocks
		done <- ok
	}()

	time.Sleep(20 * time.Millisecond)
	cancel()
	select {
	case ok := <-done:
		if ok {
			t.Fatal("expected ok=false on ctx cancel")
		}
	case <-time.After(time.Second):
		t.Fatal("ctx cancel did not unblock Next")
	}

	if !waitGoroutines(base + 1) { // +1 tolerance for runtime jitter
		t.Errorf("goroutine leak: base=%d now=%d", base, runtime.NumGoroutine())
	}
}

func TestCloseUnblocksWaiters(t *testing.T) {
	base := runtime.NumGoroutine()
	s := New(2)
	const n = 6
	var wg sync.WaitGroup
	results := make(chan bool, n)
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, ok := s.Next(context.Background())
			results <- ok
		}()
	}

	time.Sleep(30 * time.Millisecond)
	s.Close()

	wg.Wait()
	close(results)
	for ok := range results {
		if ok {
			t.Fatal("expected all waiters to get ok=false after Close")
		}
	}

	// Submit/Next after Close are no-ops / return false.
	s.Submit("x", queue.PriorityNormal)
	if s.Queued() != 0 {
		t.Fatal("Submit after Close should be a no-op")
	}
	if _, ok := s.Next(context.Background()); ok {
		t.Fatal("Next after Close should return false")
	}

	if !waitGoroutines(base + 1) {
		t.Errorf("goroutine leak after close: base=%d now=%d", base, runtime.NumGoroutine())
	}
}

func TestParsePriority(t *testing.T) {
	cases := map[string]queue.Priority{
		"low":     queue.PriorityLow,
		"LOW":     queue.PriorityLow,
		"high":    queue.PriorityHigh,
		"High":    queue.PriorityHigh,
		"normal":  queue.PriorityNormal,
		"":        queue.PriorityNormal,
		"garbage": queue.PriorityNormal,
	}
	for in, want := range cases {
		if got := ParsePriority(in); got != want {
			t.Errorf("ParsePriority(%q)=%v want %v", in, got, want)
		}
	}
}

func TestDoneUnknownIsNoOp(t *testing.T) {
	s := New(1)
	s.Done("nope") // must not panic or alter state
	if s.Running() != 0 {
		t.Fatalf("expected 0 running, got %d", s.Running())
	}
}

func TestNextNilCtx(t *testing.T) {
	s := New(1)
	s.Submit("a", queue.PriorityNormal)
	//nolint:staticcheck // intentionally exercising nil ctx path
	id, ok := s.Next(nil)
	if !ok || id != "a" {
		t.Fatalf("nil ctx Next: got %q ok=%v", id, ok)
	}
}

func TestConcurrentSubmitNextDoneNoLeak(t *testing.T) {
	base := runtime.NumGoroutine()
	s := New(4)
	ctx, cancel := context.WithCancel(context.Background())

	var produced, consumed int64
	var producers, consumers sync.WaitGroup

	for p := 0; p < 4; p++ {
		producers.Add(1)
		go func(p int) {
			defer producers.Done()
			for i := 0; i < 100; i++ {
				s.Submit(fmt.Sprintf("p%d-%d", p, i), ParsePriority([]string{"low", "normal", "high"}[i%3]))
			}
		}(p)
	}

	for w := 0; w < 6; w++ {
		consumers.Add(1)
		go func() {
			defer consumers.Done()
			for {
				id, ok := s.Next(ctx)
				if !ok {
					return
				}
				atomic.AddInt64(&consumed, 1)
				s.Done(id)
			}
		}()
	}

	// Ensure every Submit has happened before we judge the queue drained.
	producers.Wait()
	atomic.StoreInt64(&produced, 4*100)

	// Wait for the queue to drain.
	deadline := time.After(5 * time.Second)
	for {
		if s.Queued() == 0 && s.Running() == 0 {
			break
		}
		select {
		case <-deadline:
			t.Fatalf("did not drain: queued=%d running=%d", s.Queued(), s.Running())
		case <-time.After(time.Millisecond):
		}
	}

	cancel()
	s.Close()
	consumers.Wait()

	if c := atomic.LoadInt64(&consumed); c != produced {
		t.Fatalf("consumed %d want %d (dedupe should not drop unique ids)", c, produced)
	}

	if !waitGoroutines(base + 1) {
		t.Errorf("goroutine leak: base=%d now=%d", base, runtime.NumGoroutine())
	}
}
