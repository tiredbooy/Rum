// Package power keeps the machine awake while downloads run. It inhibits system
// sleep only (never display sleep) and releases the inhibition as soon as the
// last download stops.
package power

import (
	"log"
	"sync"
)

// Backend takes one OS-level sleep inhibition. Acquire returns a release func
// that must be safe to call more than once.
type Backend interface {
	Acquire(reason string) (release func(), err error)
}

const inhibitReason = "Rum is downloading files"

// Inhibitor reference-counts holders and talks to the Backend only on the 0->1
// and 1->0 transitions, so any number of concurrent downloads share a single
// OS inhibition. All methods are safe for concurrent use and on a nil receiver.
type Inhibitor struct {
	backend Backend

	mu       sync.Mutex
	enabled  bool
	closed   bool
	holds    int
	release  func()
	warnOnce sync.Once
}

func NewInhibitor(b Backend) *Inhibitor { return &Inhibitor{backend: b} }

// SetEnabled turns the feature on or off live. Disabling releases any OS
// inhibition; enabling while downloads are active takes it.
func (i *Inhibitor) SetEnabled(on bool) {
	if i == nil {
		return
	}
	i.mu.Lock()
	defer i.mu.Unlock()
	i.enabled = on
	i.syncLocked()
}

// Hold registers one running download and returns its release func (idempotent).
func (i *Inhibitor) Hold() func() {
	if i == nil {
		return func() {}
	}
	i.mu.Lock()
	i.holds++
	i.syncLocked()
	i.mu.Unlock()

	var once sync.Once
	return func() {
		once.Do(func() {
			i.mu.Lock()
			i.holds--
			i.syncLocked()
			i.mu.Unlock()
		})
	}
}

// Close releases the OS inhibition for good (app quit).
func (i *Inhibitor) Close() {
	if i == nil {
		return
	}
	i.mu.Lock()
	defer i.mu.Unlock()
	i.closed = true
	i.syncLocked()
}

// syncLocked makes the backend state match (enabled && holds>0 && !closed).
func (i *Inhibitor) syncLocked() {
	want := i.enabled && !i.closed && i.holds > 0 && i.backend != nil
	if want && i.release == nil {
		rel, err := i.backend.Acquire(inhibitReason)
		if err != nil {
			i.warnOnce.Do(func() { log.Printf("sleep inhibit unavailable: %v", err) })
			return
		}
		i.release = rel
	} else if !want && i.release != nil {
		i.release()
		i.release = nil
	}
}
