package power

import (
	"errors"
	"sync"
	"testing"
)

type fakeBackend struct {
	mu       sync.Mutex
	acquires int
	releases int
	fail     error
}

func (f *fakeBackend) Acquire(reason string) (func(), error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.fail != nil {
		return nil, f.fail
	}
	f.acquires++
	var once sync.Once
	return func() { once.Do(func() { f.mu.Lock(); f.releases++; f.mu.Unlock() }) }, nil
}

func (f *fakeBackend) counts() (int, int) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.acquires, f.releases
}

func TestInhibitorAcquiresOnceForManyHolds(t *testing.T) {
	fb := &fakeBackend{}
	in := NewInhibitor(fb)
	in.SetEnabled(true)

	r1 := in.Hold()
	r2 := in.Hold()
	r3 := in.Hold()
	if a, r := fb.counts(); a != 1 || r != 0 {
		t.Fatalf("after 3 holds want 1 acquire 0 release, got %d/%d", a, r)
	}
	r1()
	r2()
	if _, r := fb.counts(); r != 0 {
		t.Fatalf("released while a hold remains")
	}
	r3()
	if a, r := fb.counts(); a != 1 || r != 1 {
		t.Fatalf("after all released want 1/1, got %d/%d", a, r)
	}
}

func TestInhibitorReleaseIsIdempotent(t *testing.T) {
	fb := &fakeBackend{}
	in := NewInhibitor(fb)
	in.SetEnabled(true)
	r1 := in.Hold()
	r2 := in.Hold()
	r1()
	r1() // double release must not steal r2's hold
	if _, r := fb.counts(); r != 0 {
		t.Fatalf("double release dropped another holder's hold")
	}
	r2()
	if _, r := fb.counts(); r != 1 {
		t.Fatalf("want released after last hold")
	}
}

func TestInhibitorDisabledNeverAcquires(t *testing.T) {
	fb := &fakeBackend{}
	in := NewInhibitor(fb)
	in.SetEnabled(false)
	rel := in.Hold()
	if a, _ := fb.counts(); a != 0 {
		t.Fatalf("acquired while disabled")
	}
	rel()
}

func TestInhibitorToggleWhileHeld(t *testing.T) {
	fb := &fakeBackend{}
	in := NewInhibitor(fb)
	in.SetEnabled(true)
	rel := in.Hold()
	in.SetEnabled(false)
	if a, r := fb.counts(); a != 1 || r != 1 {
		t.Fatalf("disable should release: %d/%d", a, r)
	}
	in.SetEnabled(true)
	if a, _ := fb.counts(); a != 2 {
		t.Fatalf("re-enable with active hold should re-acquire, got %d", a)
	}
	rel()
	if _, r := fb.counts(); r != 2 {
		t.Fatalf("want 2 releases, got %d", r)
	}
}

func TestInhibitorCloseReleases(t *testing.T) {
	fb := &fakeBackend{}
	in := NewInhibitor(fb)
	in.SetEnabled(true)
	_ = in.Hold()
	in.Close()
	if _, r := fb.counts(); r != 1 {
		t.Fatalf("Close must release the backend")
	}
	in.Hold() // after Close no new acquisition
	if a, _ := fb.counts(); a != 1 {
		t.Fatalf("acquired after Close")
	}
}

func TestInhibitorBackendFailureIsNotFatal(t *testing.T) {
	fb := &fakeBackend{fail: errors.New("no systemd")}
	in := NewInhibitor(fb)
	in.SetEnabled(true)
	rel := in.Hold() // must not panic
	rel()
}

func TestInhibitorNilSafe(t *testing.T) {
	var in *Inhibitor
	in.Hold()()
	in.SetEnabled(true)
	in.Close()
}
