package main

import (
	"sync"
	"time"
)

// trayAction is one entry of the tray menu.
type trayAction int

const (
	trayShow trayAction = iota
	trayPauseAll
	trayResumeAll
	trayQuit
)

// trayHandler is what a tray menu click drives. The real implementation talks
// to Wails and the local API; tests use a fake.
type trayHandler interface {
	Show()
	PauseAll()
	ResumeAll()
	Quit()
}

// dispatchTrayAction routes a menu click to the handler. Unknown actions are
// ignored.
func dispatchTrayAction(a trayAction, h trayHandler) {
	switch a {
	case trayShow:
		h.Show()
	case trayPauseAll:
		h.PauseAll()
	case trayResumeAll:
		h.ResumeAll()
	case trayQuit:
		h.Quit()
	}
}

// sniHostUsable decides from a StatusNotifierWatcher probe whether a tray icon
// would actually be visible: the watcher must exist AND report a registered
// host (GNOME without the AppIndicator extension has neither).
func sniHostUsable(watcherPresent, hostRegistered bool, err error) bool {
	return err == nil && watcherPresent && hostRegistered
}

// shouldHideOnClose is the close-to-tray fallback rule: hide only when the user
// asked for it AND a tray icon is alive to restore the window from.
func shouldHideOnClose(closeToTray, trayAlive bool) bool {
	return closeToTray && trayAlive
}

// cachedProbe memoizes a boolean probe for ttl so hot paths (the 400ms minimize
// poll) do not open a D-Bus connection on every call.
type cachedProbe struct {
	mu    sync.Mutex
	ttl   time.Duration
	now   func() time.Time
	probe func() bool
	at    time.Time
	val   bool
	set   bool
}

func (c *cachedProbe) Get() bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.set && c.now().Sub(c.at) < c.ttl {
		return c.val
	}
	c.val, c.at, c.set = c.probe(), c.now(), true
	return c.val
}
