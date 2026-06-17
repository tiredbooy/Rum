package config

import (
	"testing"
)

func TestValidateClampsRanges(t *testing.T) {
	s := &Setting{
		SpeedLimitKB:   -5,
		MaxParallel:    0,
		MaxRetries:     -1,
		OutDir:         "  ",
		FileConflict:   "bogus",
		LogLevel:       "verbose",
		PreferredTheme: "",
	}
	s.PostDownload.Action = "explode"
	s.Validate()

	if s.SpeedLimitKB != 0 {
		t.Errorf("SpeedLimitKB = %d, want 0", s.SpeedLimitKB)
	}
	if s.MaxParallel != 1 {
		t.Errorf("MaxParallel = %d, want 1", s.MaxParallel)
	}
	if s.MaxRetries != 0 {
		t.Errorf("MaxRetries = %d, want 0", s.MaxRetries)
	}
	if s.OutDir == "" {
		t.Error("OutDir should be defaulted to a real dir")
	}
	if s.FileConflict != "rename" {
		t.Errorf("FileConflict = %q, want rename", s.FileConflict)
	}
	if s.LogLevel != "info" {
		t.Errorf("LogLevel = %q, want info", s.LogLevel)
	}
	if s.PostDownload.Action != "none" {
		t.Errorf("Action = %q, want none", s.PostDownload.Action)
	}
	if s.PreferredTheme != "system" {
		t.Errorf("PreferredTheme = %q, want system", s.PreferredTheme)
	}
}

func TestValidateUpperBounds(t *testing.T) {
	s := &Setting{MaxParallel: 1000, MaxRetries: 9999}
	s.Validate()
	if s.MaxParallel != 64 {
		t.Errorf("MaxParallel = %d, want 64", s.MaxParallel)
	}
	if s.MaxRetries != 100 {
		t.Errorf("MaxRetries = %d, want 100", s.MaxRetries)
	}
}

func TestValidateBandwidthSchedule(t *testing.T) {
	s := &Setting{
		BandwidthSchedule: []SpeedRule{
			{StartHour: -3, EndHour: 99, LimitKBps: -10},
		},
	}
	s.Validate()
	r := s.BandwidthSchedule[0]
	if r.StartHour != 0 || r.EndHour != 23 || r.LimitKBps != 0 {
		t.Errorf("bandwidth rule not clamped: %+v", r)
	}
}

func TestValidateAcceptsValidValues(t *testing.T) {
	s := &Setting{
		SpeedLimitKB:   500,
		MaxParallel:    8,
		MaxRetries:     5,
		OutDir:         "/tmp/x",
		FileConflict:   "overwrite",
		LogLevel:       "debug",
		PreferredTheme: "dark",
	}
	s.PostDownload.Action = "shutdown"
	s.Validate()
	if s.MaxParallel != 8 || s.FileConflict != "overwrite" ||
		s.LogLevel != "debug" || s.PostDownload.Action != "shutdown" {
		t.Errorf("valid settings were altered: %+v", s)
	}
}

// ---------------------------------------------------------------------------
// A4: SpeedRule.Days + ScheduledStartEnabled
// ---------------------------------------------------------------------------

func TestValidateClampsSpeedRuleDays(t *testing.T) {
	s := &Setting{
		BandwidthSchedule: []SpeedRule{
			// -1 and 7 are out of the 0..6 range and must be dropped; 3 stays.
			{StartHour: 0, EndHour: 8, LimitKBps: 500, Days: []int{-1, 3, 7}},
		},
	}
	s.Validate()
	got := s.BandwidthSchedule[0].Days
	if len(got) != 1 || got[0] != 3 {
		t.Fatalf("Days not clamped to valid 0..6 entries: %v", got)
	}
}

func TestScheduledStartEnabledRoundTrips(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	s := &Setting{}
	s.setDefaults()
	s.ScheduledStartEnabled = true
	if err := s.Save(); err != nil {
		t.Fatalf("save: %v", err)
	}
	var reloaded Setting
	if err := reloaded.LoadSettingMetadata(); err != nil {
		t.Fatalf("load: %v", err)
	}
	if !reloaded.ScheduledStartEnabled {
		t.Fatalf("ScheduledStartEnabled did not round-trip")
	}
}

// ---------------------------------------------------------------------------
// A5: CategoryRule + EnableCategories
// ---------------------------------------------------------------------------

func TestValidateDropsInvalidCategoryRules(t *testing.T) {
	s := &Setting{
		EnableCategories: true,
		Categories: []CategoryRule{
			{Name: "Video", Extensions: []string{".mp4", ".mkv"}, DestDir: "Videos"},
			{Name: "", Extensions: []string{".x"}, DestDir: "X"},      // no name -> dropped
			{Name: "Empty", Extensions: nil, DestDir: "Empty"},        // no exts -> dropped
			{Name: "Docs", Extensions: []string{".pdf"}, DestDir: ""}, // empty dest is allowed (defaults to current dir)
		},
	}
	s.Validate()
	if len(s.Categories) != 2 {
		t.Fatalf("expected 2 valid category rules, got %d: %+v", len(s.Categories), s.Categories)
	}
	if s.Categories[0].Name != "Video" || s.Categories[1].Name != "Docs" {
		t.Fatalf("unexpected surviving rules: %+v", s.Categories)
	}
}

func TestCategoriesRoundTrip(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	s := &Setting{}
	s.setDefaults()
	s.EnableCategories = true
	s.Categories = []CategoryRule{{Name: "Audio", Extensions: []string{".mp3"}, DestDir: "Music"}}
	if err := s.Save(); err != nil {
		t.Fatalf("save: %v", err)
	}
	var reloaded Setting
	if err := reloaded.LoadSettingMetadata(); err != nil {
		t.Fatalf("load: %v", err)
	}
	if !reloaded.EnableCategories || len(reloaded.Categories) != 1 || reloaded.Categories[0].Name != "Audio" {
		t.Fatalf("categories did not round-trip: %+v", reloaded.Categories)
	}
}

// ---------------------------------------------------------------------------
// A7: Desktop preference fields
// ---------------------------------------------------------------------------

func TestDesktopFieldsDefaultFalse(t *testing.T) {
	s := &Setting{}
	s.setDefaults()
	if s.LaunchOnStartup || s.MinimizeToTray || s.CloseToTray || s.EnableClipboardWatch {
		t.Fatalf("desktop toggles should default to false: %+v", s)
	}
}

func TestDesktopFieldsRoundTrip(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())

	s := &Setting{}
	s.setDefaults()
	s.LaunchOnStartup = true
	s.MinimizeToTray = true
	s.CloseToTray = true
	s.EnableClipboardWatch = true
	s.WindowState = WindowState{W: 1280, H: 800, X: 100, Y: 50, Maximized: true}

	if err := s.Save(); err != nil {
		t.Fatalf("save: %v", err)
	}

	var reloaded Setting
	if err := reloaded.LoadSettingMetadata(); err != nil {
		t.Fatalf("load: %v", err)
	}
	if !reloaded.LaunchOnStartup || !reloaded.MinimizeToTray ||
		!reloaded.CloseToTray || !reloaded.EnableClipboardWatch {
		t.Fatalf("desktop booleans did not round-trip: %+v", reloaded)
	}
	want := WindowState{W: 1280, H: 800, X: 100, Y: 50, Maximized: true}
	if reloaded.WindowState != want {
		t.Fatalf("WindowState round-trip = %+v, want %+v", reloaded.WindowState, want)
	}
}

func TestUpdateTogglesDesktopFields(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())

	var s Setting
	s.setDefaults()
	if err := s.Save(); err != nil {
		t.Fatalf("save: %v", err)
	}

	yes := true
	if err := s.Update(SettingReq{MinimizeToTray: &yes, CloseToTray: &yes}); err != nil {
		t.Fatalf("update: %v", err)
	}
	if !s.MinimizeToTray || !s.CloseToTray {
		t.Fatalf("Update did not apply desktop toggles: %+v", s)
	}

	var reloaded Setting
	if err := reloaded.LoadSettingMetadata(); err != nil {
		t.Fatalf("load: %v", err)
	}
	if !reloaded.MinimizeToTray || !reloaded.CloseToTray {
		t.Fatalf("desktop toggles not persisted by Update: %+v", reloaded)
	}
}
