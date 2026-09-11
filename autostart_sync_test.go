package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

// writeLaunchPref persists a settings.json carrying just the launch-on-startup
// preference, in an isolated config dir (settingsPath honours os.UserConfigDir,
// which XDG_CONFIG_HOME drives on Linux).
func writeLaunchPref(t *testing.T, enabled bool) {
	t.Helper()
	path := settingsPath()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	body, _ := json.Marshal(map[string]bool{"launch_on_startup": enabled})
	if err := os.WriteFile(path, body, 0o644); err != nil {
		t.Fatalf("write settings: %v", err)
	}
}

// autostartEntryExists reports whether the OS login entry is currently present.
func autostartEntryExists(t *testing.T) bool {
	t.Helper()
	dir, err := autostartDir()
	if err != nil {
		t.Fatalf("autostartDir: %v", err)
	}
	_, statErr := os.Stat(filepath.Join(dir, autostartFileName))
	return statErr == nil
}

// Toggling "Launch on startup" in the settings UI writes the preference over the
// HTTP API; before the watcher existed, reconcileAutostart only ran at launch, so
// the switch persisted and showed "Saved" while the OS login entry never changed
// until the next restart.
func TestSyncAutostartAppliesAChangedPreference(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())

	writeLaunchPref(t, true)
	if got := syncAutostart(false); !got {
		t.Fatal("syncAutostart did not adopt the enabled preference")
	}
	if !autostartEntryExists(t) {
		t.Fatal("autostart entry was not created")
	}

	writeLaunchPref(t, false)
	if got := syncAutostart(true); got {
		t.Fatal("syncAutostart did not adopt the disabled preference")
	}
	if autostartEntryExists(t) {
		t.Fatal("autostart entry was not removed")
	}
}

// An unchanged preference must not touch the OS entry at all.
func TestSyncAutostartIsANoOpWhenUnchanged(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	writeLaunchPref(t, false)

	if got := syncAutostart(false); got {
		t.Fatal("syncAutostart flipped state for an unchanged preference")
	}
	if autostartEntryExists(t) {
		t.Fatal("autostart entry created for a disabled preference")
	}
}

// A missing settings file must not crash the watcher; it reads as "off".
func TestSyncAutostartToleratesMissingSettings(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	if got := syncAutostart(false); got {
		t.Fatal("expected the default (off) with no settings file")
	}
}

// The desktop capability matrix must report the tray honestly, so the settings
// UI can disable the tray switches instead of offering inert ones.
func TestCapabilitiesReportsTrayAvailability(t *testing.T) {
	caps := NewApp().Capabilities()
	if caps.Tray != trayAvailable {
		t.Fatalf("Capabilities().Tray = %v, want %v", caps.Tray, trayAvailable)
	}
	if caps.Platform == "" {
		t.Fatal("Capabilities().Platform is empty")
	}
	// No Wails context in a unit test, so the picker is correctly reported absent.
	if caps.FolderPicker {
		t.Fatal("FolderPicker should be false before startup wires the context")
	}
}

// ChooseDir before startup must return a clear error rather than an empty string
// the frontend cannot tell apart from a cancel.
func TestChooseDirBeforeStartupReturnsAnError(t *testing.T) {
	dir, err := NewApp().ChooseDir()
	if err == nil {
		t.Fatal("expected an error when the window is not ready")
	}
	if dir != "" {
		t.Fatalf("dir = %q, want empty", dir)
	}
}
