package queue

import (
	"context"
	"sync"
	"testing"
	"time"
)

func TestPriorityOrdering(t *testing.T) {
	pq := NewPriorityQueue(0)
	ctx := context.Background()

	// Push mixed priorities.
	_ = pq.Push(ctx, "low1", PriorityLow)
	_ = pq.Push(ctx, "high1", PriorityHigh)
	_ = pq.Push(ctx, "normal1", PriorityNormal)
	_ = pq.Push(ctx, "high2", PriorityHigh)
	_ = pq.Push(ctx, "low2", PriorityLow)

	want := []string{"high1", "high2", "normal1", "low1", "low2"}
	for i, w := range want {
		v, err := pq.Pop(ctx)
		if err != nil {
			t.Fatalf("pop %d: %v", i, err)
		}
		if v.(string) != w {
			t.Fatalf("pop %d: got %q want %q", i, v, w)
		}
	}
}

func TestFIFOWithinPriority(t *testing.T) {
	pq := NewPriorityQueue(0)
	ctx := context.Background()
	for i := 0; i < 100; i++ {
		_ = pq.Push(ctx, i, PriorityNormal)
	}
	for i := 0; i < 100; i++ {
		v, err := pq.Pop(ctx)
		if err != nil {
			t.Fatal(err)
		}
		if v.(int) != i {
			t.Fatalf("FIFO violated: got %d want %d", v, i)
		}
	}
}

func TestPopBlocksUntilPush(t *testing.T) {
	pq := NewPriorityQueue(0)
	ctx := context.Background()
	got := make(chan any, 1)
	go func() {
		v, err := pq.Pop(ctx)
		if err != nil {
			t.Errorf("pop: %v", err)
			return
		}
		got <- v
	}()
	// Give the popper time to block.
	time.Sleep(20 * time.Millisecond)
	_ = pq.Push(ctx, "x", PriorityNormal)
	select {
	case v := <-got:
		if v.(string) != "x" {
			t.Fatalf("got %v", v)
		}
	case <-time.After(time.Second):
		t.Fatal("pop did not unblock")
	}
}

func TestPopContextCancel(t *testing.T) {
	pq := NewPriorityQueue(0)
	ctx, cancel := context.WithCancel(context.Background())
	errc := make(chan error, 1)
	go func() {
		_, err := pq.Pop(ctx)
		errc <- err
	}()
	time.Sleep(20 * time.Millisecond)
	cancel()
	select {
	case err := <-errc:
		if err != context.Canceled {
			t.Fatalf("got %v want context.Canceled", err)
		}
	case <-time.After(time.Second):
		t.Fatal("pop did not return on cancel (goroutine leak)")
	}
}

func TestPopClosedDrainsThenErrors(t *testing.T) {
	pq := NewPriorityQueue(0)
	ctx := context.Background()
	_ = pq.Push(ctx, "a", PriorityNormal)
	pq.Close()
	// Still drains the queued item.
	v, err := pq.Pop(ctx)
	if err != nil || v.(string) != "a" {
		t.Fatalf("drain: v=%v err=%v", v, err)
	}
	// Now empty + closed -> ErrQueueClosed.
	if _, err := pq.Pop(ctx); err != ErrQueueClosed {
		t.Fatalf("got %v want ErrQueueClosed", err)
	}
	// Push on closed queue fails.
	if err := pq.Push(ctx, "b", PriorityNormal); err != ErrQueueClosed {
		t.Fatalf("push closed: got %v", err)
	}
}

func TestCloseWakesBlockedPop(t *testing.T) {
	pq := NewPriorityQueue(0)
	ctx := context.Background()
	errc := make(chan error, 1)
	go func() {
		_, err := pq.Pop(ctx)
		errc <- err
	}()
	time.Sleep(20 * time.Millisecond)
	pq.Close()
	select {
	case err := <-errc:
		if err != ErrQueueClosed {
			t.Fatalf("got %v want ErrQueueClosed", err)
		}
	case <-time.After(time.Second):
		t.Fatal("Close did not wake blocked Pop")
	}
}

func TestBoundedPushBlocks(t *testing.T) {
	pq := NewPriorityQueue(2)
	ctx := context.Background()
	if ok, _ := pq.TryPush(1, PriorityNormal); !ok {
		t.Fatal("first TryPush should succeed")
	}
	if ok, _ := pq.TryPush(2, PriorityNormal); !ok {
		t.Fatal("second TryPush should succeed")
	}
	if ok, _ := pq.TryPush(3, PriorityNormal); ok {
		t.Fatal("third TryPush should fail (full)")
	}

	pushed := make(chan struct{})
	go func() {
		_ = pq.Push(ctx, 3, PriorityNormal)
		close(pushed)
	}()
	time.Sleep(20 * time.Millisecond)
	select {
	case <-pushed:
		t.Fatal("Push should block while full")
	default:
	}
	// Free a slot.
	if _, _, err := pq.TryPop(); err != nil {
		t.Fatal(err)
	}
	select {
	case <-pushed:
	case <-time.After(time.Second):
		t.Fatal("Push did not unblock after slot freed")
	}
}

func TestBoundedPushContextCancel(t *testing.T) {
	pq := NewPriorityQueue(1)
	ctx, cancel := context.WithCancel(context.Background())
	_, _ = pq.TryPush(1, PriorityNormal)
	errc := make(chan error, 1)
	go func() { errc <- pq.Push(ctx, 2, PriorityNormal) }()
	time.Sleep(20 * time.Millisecond)
	cancel()
	select {
	case err := <-errc:
		if err != context.Canceled {
			t.Fatalf("got %v want context.Canceled", err)
		}
	case <-time.After(time.Second):
		t.Fatal("blocked Push did not return on cancel")
	}
}

func TestDrain(t *testing.T) {
	pq := NewPriorityQueue(0)
	ctx := context.Background()
	_ = pq.Push(ctx, "low", PriorityLow)
	_ = pq.Push(ctx, "high", PriorityHigh)
	out := pq.Drain()
	if len(out) != 2 || out[0].(string) != "high" || out[1].(string) != "low" {
		t.Fatalf("drain order wrong: %v", out)
	}
	if pq.Len() != 0 {
		t.Fatal("queue not empty after drain")
	}
}

// TestConcurrentProducersConsumers stresses the queue with many goroutines to
// surface races under -race and confirm no items are lost or duplicated.
func TestConcurrentProducersConsumers(t *testing.T) {
	pq := NewPriorityQueue(0)
	ctx := context.Background()
	const producers = 8
	const perProducer = 1000
	total := producers * perProducer

	var wg sync.WaitGroup
	for p := 0; p < producers; p++ {
		wg.Add(1)
		go func(base int) {
			defer wg.Done()
			for i := 0; i < perProducer; i++ {
				if err := pq.Push(ctx, base+i, PriorityNormal); err != nil {
					t.Errorf("push: %v", err)
					return
				}
			}
		}(p * perProducer)
	}

	var mu sync.Mutex
	seen := make(map[int]bool, total)
	var cwg sync.WaitGroup
	const consumers = 4
	done := make(chan struct{})
	for c := 0; c < consumers; c++ {
		cwg.Add(1)
		go func() {
			defer cwg.Done()
			for {
				select {
				case <-done:
					return
				default:
				}
				v, ok, err := pq.TryPop()
				if err != nil {
					return
				}
				if !ok {
					time.Sleep(time.Millisecond)
					continue
				}
				mu.Lock()
				n := v.(int)
				if seen[n] {
					t.Errorf("duplicate item %d", n)
				}
				seen[n] = true
				mu.Unlock()
			}
		}()
	}

	wg.Wait()
	// Wait until everything is consumed.
	deadline := time.After(5 * time.Second)
	for {
		mu.Lock()
		n := len(seen)
		mu.Unlock()
		if n == total {
			break
		}
		select {
		case <-deadline:
			t.Fatalf("only consumed %d/%d", n, total)
		case <-time.After(2 * time.Millisecond):
		}
	}
	close(done)
	cwg.Wait()
}
