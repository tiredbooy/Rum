package main

import "sync"

// windowVisibility records whether the user can currently see the Rum window.
// Wails v2 has no window-state callbacks, so the state is fed from the places
// that change it: tray hide/show call sites, the minimise watcher, and the
// frontend's visibilitychange report (SetWindowVisible) as a fallback.
type windowVisibility struct {
	mu         sync.Mutex
	hidden     bool // hidden to the tray
	minimised  bool
	pageHidden bool // frontend reported document.hidden
}

func (v *windowVisibility) SetHidden(b bool)      { v.set(&v.hidden, b) }
func (v *windowVisibility) SetMinimised(b bool)   { v.set(&v.minimised, b) }
func (v *windowVisibility) SetPageVisible(b bool) { v.set(&v.pageHidden, !b) }

func (v *windowVisibility) set(f *bool, val bool) {
	v.mu.Lock()
	*f = val
	v.mu.Unlock()
}

// Shown clears every source; call it wherever the window is raised.
func (v *windowVisibility) Shown() {
	v.mu.Lock()
	v.hidden, v.minimised, v.pageHidden = false, false, false
	v.mu.Unlock()
}

// Hidden reports whether any source says the user cannot see the window.
func (v *windowVisibility) Hidden() bool {
	v.mu.Lock()
	defer v.mu.Unlock()
	return v.hidden || v.minimised || v.pageHidden
}
