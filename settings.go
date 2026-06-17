package main

import (
	"encoding/json"
	"os"
	"path/filepath"
)

// desktopSettings is a read/write view of the subset of the backend's
// settings.json that the desktop layer needs.
//
// WHY this exists: the root Wails module cannot import
// github.com/tiredbooy/Rum/backend/internal/pkg/config (Go's internal-package
// rule), and the only public accessor the backend exposes — server.LoadSettings()
// — returns a PublicSettings struct that exposes just ConfirmOnExit (CloseToTray
// is declared but never populated from disk). That is not enough to drive
// close-to-tray, minimize-to-tray, autostart, the clipboard watcher, or
// window-state persistence, and PublicSettings has no Save().
//
// Rather than break the engine's internal boundary, the desktop layer reads and
// writes the SAME settings.json the backend uses, in the SAME location
// (os.UserConfigDir()/rum/settings.json — see
// backend/internal/pkg/file-system/generateMetadata.go), using a struct whose
// json tags mirror config.Setting exactly. Load() round-trips through a
// json.RawMessage map so we only touch the desktop-owned keys and never clobber
// fields written by the backend (download limits, categories, schedule, etc.).
//
// This is recorded as a "requested upstream change" in the agent report: ideally
// the backend would widen server.LoadSettings()/add a typed accessor + saver for
// these fields so the desktop layer would not need to know the on-disk layout.
type desktopSettings struct {
	// raw holds every key in settings.json so Save() can write back the desktop
	// fields without dropping anything the backend owns.
	raw map[string]json.RawMessage

	ConfirmOnExit        bool
	Silent               bool
	OutDir               string
	LaunchOnStartup      bool
	MinimizeToTray       bool
	CloseToTray          bool
	EnableClipboardWatch bool
	WindowState          windowState
}

// windowState mirrors config.WindowState (json tags w/h/x/y/maximized).
type windowState struct {
	W         int  `json:"w"`
	H         int  `json:"h"`
	X         int  `json:"x"`
	Y         int  `json:"y"`
	Maximized bool `json:"maximized"`
}

// settingsPath returns os.UserConfigDir()/rum/settings.json, matching
// backend/internal/pkg/file-system.CreateMetadataFile. It does not create the
// directory; Save() does that.
func settingsPath() string {
	configDir, err := os.UserConfigDir()
	if err != nil || configDir == "" {
		configDir = ".rum"
	}
	return filepath.Join(configDir, "rum", "settings.json")
}

// loadDesktopSettings reads the desktop-relevant fields from settings.json. A
// missing or unparseable file yields zero-value defaults (all toggles off) and a
// nil error, so the desktop layer degrades gracefully and never blocks startup.
func loadDesktopSettings() *desktopSettings {
	ds := &desktopSettings{raw: map[string]json.RawMessage{}}

	data, err := os.ReadFile(settingsPath())
	if err != nil {
		return ds // defaults; file may not exist yet (backend writes it on first run)
	}
	if err := json.Unmarshal(data, &ds.raw); err != nil {
		return ds // corrupt file: fall back to defaults rather than crashing
	}

	unmarshalBool(ds.raw, "confirm_on_exit", &ds.ConfirmOnExit)
	unmarshalBool(ds.raw, "silent", &ds.Silent)
	unmarshalString(ds.raw, "out_dir", &ds.OutDir)
	unmarshalBool(ds.raw, "launch_on_startup", &ds.LaunchOnStartup)
	unmarshalBool(ds.raw, "minimize_to_tray", &ds.MinimizeToTray)
	unmarshalBool(ds.raw, "close_to_tray", &ds.CloseToTray)
	unmarshalBool(ds.raw, "enable_clipboard_watch", &ds.EnableClipboardWatch)
	if v, ok := ds.raw["window_state"]; ok {
		_ = json.Unmarshal(v, &ds.WindowState)
	}
	return ds
}

// SaveWindowState persists only the window geometry back to settings.json,
// preserving every other key the backend owns. It re-reads the file first so a
// concurrent backend write of a non-window field is not lost.
func (ds *desktopSettings) SaveWindowState(ws windowState) error {
	ds.WindowState = ws

	// Re-read so we merge onto the freshest on-disk state (best effort).
	raw := map[string]json.RawMessage{}
	if data, err := os.ReadFile(settingsPath()); err == nil {
		_ = json.Unmarshal(data, &raw)
	}
	if raw == nil {
		raw = map[string]json.RawMessage{}
	}

	encoded, err := json.Marshal(ws)
	if err != nil {
		return err
	}
	raw["window_state"] = encoded

	out, err := json.MarshalIndent(raw, "", "  ")
	if err != nil {
		return err
	}

	path := settingsPath()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	// Atomic write: temp file + rename so a crash mid-write can't corrupt the
	// shared settings.json.
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, out, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

func unmarshalBool(raw map[string]json.RawMessage, key string, dst *bool) {
	if v, ok := raw[key]; ok {
		_ = json.Unmarshal(v, dst)
	}
}

func unmarshalString(raw map[string]json.RawMessage, key string, dst *string) {
	if v, ok := raw[key]; ok {
		_ = json.Unmarshal(v, dst)
	}
}
