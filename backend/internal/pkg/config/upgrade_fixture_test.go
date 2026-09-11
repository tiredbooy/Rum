package config

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

// oldBuildSettingsJSON is a byte-accurate settings.json as the PREVIOUS build's
// Save() produced it: every field of the old struct, written explicitly, with the
// old defaults — and crucially no settings_version key.
//
// This is the fixture that matters. Reasoning about "a key will be absent" is
// how an upgrade regression slips through: Save always wrote the whole struct,
// so for a real upgrader NO key is ever absent.
const oldBuildSettingsJSON = `{
  "start_on_launch": true,
  "confirm_on_exit": true,
  "silent": false,
  "out_dir": "/tmp/rum-old-out",
  "speed_limit_kb": 0,
  "max_parallel": 1,
  "max_retries": 3,
  "connections": 8,
  "preferred_theme": "system",
  "scheduled_start_enabled": false,
  "post_download": { "action": "none", "auto_open_dir": false },
  "file_confilict": "rename",
  "log_level": "info",
  "enable_categories": false,
  "verify_integrity": true,
  "auto_resume_on_reconnect": true,
  "auto_resume_on_launch": true,
  "retry_backoff_sec": 1,
  "accent_color": "",
  "ui_density": "comfortable",
  "reduced_motion": false,
  "temp_dir": "",
  "keep_partial_on_failure": true,
  "launch_on_startup": false,
  "minimize_to_tray": false,
  "close_to_tray": false,
  "enable_clipboard_watch": false,
  "window_state": { "w": 0, "h": 0, "x": 0, "y": 0, "maximized": false }
}`

func loadFixture(t *testing.T, body string) (Setting, string) {
	t.Helper()
	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", dir)
	path := filepath.Join(dir, "rum", "settings.json")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}

	var s Setting
	if err := s.LoadSettingMetadata(); err != nil {
		t.Fatalf("load: %v", err)
	}
	return s, path
}

// A file from the previous build must load with every user-visible preference
// unchanged, and must not silently change what the app DOES.
func TestOldBuildSettingsFileUpgradesWithoutBehaviourChange(t *testing.T) {
	s, _ := loadFixture(t, oldBuildSettingsJSON)

	// Preferences the user actually set keep their values.
	checks := []struct {
		name string
		got  any
		want any
	}{
		{"confirm_on_exit", s.ConfirmOnExit, true},
		{"silent", s.Silent, false},
		{"out_dir", s.OutDir, "/tmp/rum-old-out"},
		{"speed_limit_kb", s.SpeedLimitKB, 0},
		{"max_parallel", s.MaxParallel, 1},
		{"max_retries", s.MaxRetries, 3},
		{"connections", s.Connections, 8},
		{"preferred_theme", s.PreferredTheme, "system"},
		{"file_confilict", s.FileConflict, "rename"},
		{"log_level", s.LogLevel, "info"},
		{"enable_categories", s.EnableCategories, false},
		{"verify_integrity", s.VerifyIntegrity, true},
		{"auto_resume_on_reconnect", s.AutoResumeOnReconnect, true},
		{"auto_resume_on_launch", s.AutoResumeOnLaunch, true},
		{"retry_backoff_sec", s.RetryBackoffSec, 1},
		{"ui_density", s.UIDensity, "comfortable"},
		{"reduced_motion", s.ReducedMotion, false},
		{"temp_dir", s.TempDir, ""},
		{"keep_partial_on_failure", s.KeepPartialOnFailure, true},
		{"launch_on_startup", s.LaunchOnStartup, false},
		{"minimize_to_tray", s.MinimizeToTray, false},
		{"close_to_tray", s.CloseToTray, false},
		{"enable_clipboard_watch", s.EnableClipboardWatch, false},
		{"post_download.action", s.PostDownload.Action, "none"},
	}
	for _, c := range checks {
		if c.got != c.want {
			t.Errorf("%s = %v, want %v", c.name, c.got, c.want)
		}
	}

	// The old build ignored scheduled_start_enabled and ALWAYS started due
	// scheduled jobs. The migration must preserve that, not adopt the stored
	// false and quietly stop starting them.
	if !s.ScheduledStartEnabled {
		t.Error("scheduled_start_enabled did not migrate to the behaviour the old build actually had")
	}
	if s.SettingsVersion != currentSettingsVersion {
		t.Errorf("SettingsVersion = %d, want %d", s.SettingsVersion, currentSettingsVersion)
	}
	// The deprecated alias follows the canonical field, and both are true here.
	if !s.StartOnLaunch || s.StartOnLaunch != s.AutoResumeOnLaunch {
		t.Errorf("start_on_launch=%v auto_resume_on_launch=%v, want mirrored", s.StartOnLaunch, s.AutoResumeOnLaunch)
	}
}

// An OLD file where the user had deliberately disabled a default-true reliability
// flag must keep it disabled — the presentKeys fix must not resurrect it.
func TestOldBuildFileKeepsDeliberatelyDisabledFlags(t *testing.T) {
	var obj map[string]any
	if err := json.Unmarshal([]byte(oldBuildSettingsJSON), &obj); err != nil {
		t.Fatalf("fixture parse: %v", err)
	}
	obj["verify_integrity"] = false
	obj["auto_resume_on_launch"] = false
	obj["keep_partial_on_failure"] = false
	body, _ := json.Marshal(obj)

	s, _ := loadFixture(t, string(body))

	if s.VerifyIntegrity || s.AutoResumeOnLaunch || s.KeepPartialOnFailure {
		t.Fatalf("a deliberately disabled flag was resurrected: %+v", s)
	}
	// ...and the alias follows the canonical field down to false.
	if s.StartOnLaunch {
		t.Error("start_on_launch did not follow auto_resume_on_launch=false")
	}
}

// Once migrated and saved, a later load must respect the user's choice instead of
// re-running the migration on every launch.
func TestMigrationRunsOnceAndThenRespectsTheUsersChoice(t *testing.T) {
	s, path := loadFixture(t, oldBuildSettingsJSON)
	if !s.ScheduledStartEnabled {
		t.Fatal("precondition: migration should have enabled it")
	}
	if err := s.Save(); err != nil {
		t.Fatalf("save: %v", err)
	}

	// The user now deliberately turns it off in the settings UI.
	off := false
	if err := s.Update(SettingReq{}); err != nil {
		t.Fatalf("update: %v", err)
	}
	s.ScheduledStartEnabled = off
	if err := s.Save(); err != nil {
		t.Fatalf("save: %v", err)
	}

	var reloaded Setting
	if err := reloaded.LoadSettingMetadata(); err != nil {
		t.Fatalf("reload: %v", err)
	}
	if reloaded.ScheduledStartEnabled {
		t.Fatal("the migration re-ran and overrode the user's choice")
	}

	// The version stamp is what stops it re-running.
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	var obj map[string]any
	if err := json.Unmarshal(raw, &obj); err != nil {
		t.Fatalf("parse: %v", err)
	}
	if obj["settings_version"] != float64(currentSettingsVersion) {
		t.Fatalf("settings_version = %v, want %d", obj["settings_version"], currentSettingsVersion)
	}
}

// A brand-new install is written at the current version, so the migration never
// touches it.
func TestFreshInstallIsAlreadyAtTheCurrentVersion(t *testing.T) {
	var s Setting
	s.setDefaults()
	if s.SettingsVersion != currentSettingsVersion {
		t.Fatalf("SettingsVersion = %d, want %d", s.SettingsVersion, currentSettingsVersion)
	}
	if !s.ScheduledStartEnabled {
		t.Error("a fresh install should start scheduled downloads")
	}
}
