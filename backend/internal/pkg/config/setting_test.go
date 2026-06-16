package config

import "testing"

func TestValidateClampsRanges(t *testing.T) {
	s := &Setting{
		SpeedLimitKB: -5,
		MaxParallel:  0,
		MaxRetries:   -1,
		OutDir:       "  ",
		FileConflict: "bogus",
		LogLevel:     "verbose",
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
