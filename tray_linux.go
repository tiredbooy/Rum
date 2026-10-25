//go:build linux

package main

import (
	"context"
	_ "embed"
	"log"
	"sync/atomic"
	"time"

	"fyne.io/systray"
	"github.com/godbus/dbus/v5"
	"github.com/tiredbooy/Rum/backend/nodesktop"
	"github.com/wailsapp/wails/v2/pkg/runtime"
)

// Linux tray: StatusNotifierItem over D-Bus via fyne.io/systray. Unlike
// getlantern/systray (GTK), fyne.io/systray on Linux is pure Go over godbus and
// starts no GTK main loop, so it coexists with Wails' own GTK loop.

//go:embed frontend/src/assets/tray-icon.png
var trayIcon []byte

const sniProbeTimeout = 800 * time.Millisecond

// trayStarted is set once the tray was created because an SNI host existed.
var trayStarted atomic.Bool

// trayAvailable is true only while an SNI host is registered. It is re-probed
// on every call (cheap D-Bus property read) so closing to tray never hides the
// window when the host has vanished since launch.
func trayAvailable() bool {
	return trayStarted.Load() && hostProbe.Get()
}

var hostProbe = &cachedProbe{ttl: 3 * time.Second, now: time.Now, probe: probeSNIHost}

// probeSNIHost asks the session bus whether a StatusNotifierHost is registered.
func probeSNIHost() bool {
	if nodesktop.Blocked() {
		return false
	}
	conn, err := dbus.ConnectSessionBus()
	if err != nil {
		return sniHostUsable(false, false, err)
	}
	defer conn.Close()

	ctx, cancel := context.WithTimeout(context.Background(), sniProbeTimeout)
	defer cancel()

	var owned bool
	err = conn.BusObject().CallWithContext(ctx, "org.freedesktop.DBus.NameHasOwner", 0,
		"org.kde.StatusNotifierWatcher").Store(&owned)
	if err != nil || !owned {
		return sniHostUsable(owned, false, err)
	}
	var v dbus.Variant
	err = conn.Object("org.kde.StatusNotifierWatcher", "/StatusNotifierWatcher").
		CallWithContext(ctx, "org.freedesktop.DBus.Properties.Get", 0,
			"org.kde.StatusNotifierWatcher", "IsStatusNotifierHostRegistered").Store(&v)
	registered, _ := v.Value().(bool)
	return sniHostUsable(true, registered, err)
}

type linuxTray struct {
	ctx     context.Context
	apiBase string
}

func (t linuxTray) Show() {
	windowVis.Shown()
	runtime.WindowShow(t.ctx)
	runtime.WindowUnminimise(t.ctx)
}
func (t linuxTray) PauseAll()  { postNoBody(t.apiBase, "/api/v1/downloads/pause-all") }
func (t linuxTray) ResumeAll() { postNoBody(t.apiBase, "/api/v1/downloads/start-all") }
func (t linuxTray) Quit()      { quitFromTray(t.ctx) }

// startTray creates the tray when an SNI host exists; otherwise it does nothing
// and trayAvailable() stays false, so close/minimize keep their normal behavior.
func startTray(ctx context.Context, apiBase string) {
	if !probeSNIHost() {
		log.Println("tray: no StatusNotifier host on the session bus; tray disabled")
		return
	}
	trayStarted.Store(true)
	h := linuxTray{ctx: ctx, apiBase: apiBase}

	go systray.Run(func() {
		systray.SetIcon(trayIcon)
		systray.SetTitle("Rum")
		systray.SetTooltip("Rum Download Manager")

		items := []struct {
			item   *systray.MenuItem
			action trayAction
		}{
			{systray.AddMenuItem("Show Rum", "Bring the app to front"), trayShow},
			{systray.AddMenuItem("Pause all", "Pause all downloads"), trayPauseAll},
			{systray.AddMenuItem("Resume all", "Resume all downloads"), trayResumeAll},
			{systray.AddMenuItem("Quit", "Close the application"), trayQuit},
		}
		for _, it := range items {
			go func() {
				for {
					select {
					case <-ctx.Done():
						return
					case <-it.item.ClickedCh:
						dispatchTrayAction(it.action, h)
					}
				}
			}()
		}
	}, nil)
}
