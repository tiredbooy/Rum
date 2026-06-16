package config

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"

	filesystem "github.com/tiredbooy/Rum/backend/internal/pkg/file-system"
)

type Setting struct {
	// App behaviour
	StartOnLaunch bool `json:"start_on_launch"`
	ConfirmOnExit bool `json:"confirm_on_exit"`
	Silent        bool `json:"silent"`

	// Download limits
	OutDir       string `json:"out_dir"`
	SpeedLimitKB int    `json:"speed_limit_kb"`
	MaxParallel  int    `json:"max_parallel"`
	MaxRetries   int    `json:"max_retries"`

	// UI / Frontend
	PreferredTheme string `json:"preferred_theme"`

	// Advanced (customizable)
	BandwidthSchedule []SpeedRule `json:"bandwidth_schedule,omitempty"`
	PostDownload      struct {
		Action      string `json:"action"` // "none", "shutdown", "sleep", "close"
		AutoOpenDir bool   `json:"auto_open_dir"`
	} `json:"post_download,omitempty"`
	FileConflict string `json:"file_confilict"` // "rename", "overwrite", "skip"
	Proxy        string `json:"proxy,omitempty"`
	LogLevel     string `json:"log_level"` // "info", "debug"
}

type SettingReq struct {
	// App behaviour
	StartOnLaunch *bool `json:"start_on_launch"`
	ConfirmOnExit *bool `json:"confirm_on_exit"`
	Silent        *bool `json:"silent"`

	// Download limits
	OutDir       *string `json:"out_dir"`
	SpeedLimitKB *int    `json:"speed_limit_kb"`
	MaxParallel  *int    `json:"max_parallel"`
	MaxRetries   *int    `json:"max_retries"`

	// UI / Frontend
	PreferredTheme *string `json:"preferred_theme"`

	PostDownload struct {
		Action      *string `json:"action"` // "none", "shutdown", "sleep", "close"
		AutoOpenDir *bool   `json:"auto_open_dir"`
	} `json:"post_download,omitempty"`
	FileConflict *string `json:"file_confilict"` // "rename", "overwrite", "skip"
	Proxy        *string `json:"proxy,omitempty"`
}

type SpeedRule struct {
	StartHour int `json:"start_hour"` // 0-23
	EndHour   int `json:"end_hour"`
	LimitKBps int `json:"limit_kbps"` // 0 = unlimited
}

// validActions / validConflicts / validLogLevels define the accepted enum values.
var (
	validActions   = map[string]bool{"none": true, "shutdown": true, "sleep": true, "close": true}
	validConflicts = map[string]bool{"rename": true, "overwrite": true, "skip": true}
	validLogLevels = map[string]bool{"info": true, "debug": true, "warn": true, "error": true}
)

func (s *Setting) LoadSettingMetadata() error {
	data, err := filesystem.ReadMetadataFile("settings.json")
	if err != nil {
		if os.IsNotExist(err) {
			s.setDefaults()
			return s.Save()
		}
		// Don't crash the app on an unreadable config: fall back to defaults and
		// rewrite a clean file.
		s.setDefaults()
		_ = s.Save()
		return fmt.Errorf("read settings (using defaults): %w", err)
	}

	if err := json.Unmarshal(data, s); err != nil {
		// Corrupt/partial config: recover with defaults instead of failing hard.
		s.setDefaults()
		_ = s.Save()
		return fmt.Errorf("parse settings (using defaults): %w", err)
	}

	s.applyMissingDefaults()
	s.Validate() // clamp out-of-range values to safe defaults
	return nil
}

// Validate clamps every field to a sane range and replaces invalid enum values
// with safe defaults. It mutates the receiver and is safe to call repeatedly.
func (s *Setting) Validate() {
	if s.SpeedLimitKB < 0 {
		s.SpeedLimitKB = 0 // 0 = unlimited
	}
	if s.MaxParallel < 1 {
		s.MaxParallel = 1
	}
	if s.MaxParallel > 64 {
		s.MaxParallel = 64
	}
	if s.MaxRetries < 0 {
		s.MaxRetries = 0
	}
	if s.MaxRetries > 100 {
		s.MaxRetries = 100
	}
	if strings.TrimSpace(s.OutDir) == "" {
		s.OutDir = filesystem.GetOrCreateDownloadDirectory()
	}
	if !validActions[s.PostDownload.Action] {
		s.PostDownload.Action = "none"
	}
	if !validConflicts[s.FileConflict] {
		s.FileConflict = "rename"
	}
	if !validLogLevels[s.LogLevel] {
		s.LogLevel = "info"
	}
	if s.PreferredTheme == "" {
		s.PreferredTheme = "system"
	}
	// Clamp bandwidth schedule hours/limits.
	for i := range s.BandwidthSchedule {
		r := &s.BandwidthSchedule[i]
		if r.StartHour < 0 {
			r.StartHour = 0
		}
		if r.StartHour > 23 {
			r.StartHour = 23
		}
		if r.EndHour < 0 {
			r.EndHour = 0
		}
		if r.EndHour > 23 {
			r.EndHour = 23
		}
		if r.LimitKBps < 0 {
			r.LimitKBps = 0
		}
	}
}

func (s *Setting) Update(req SettingReq) error {
	if req.StartOnLaunch != nil {
		s.StartOnLaunch = *req.StartOnLaunch
	}
	if req.ConfirmOnExit != nil {
		s.ConfirmOnExit = *req.ConfirmOnExit
	}
	if req.Silent != nil {
		s.Silent = *req.Silent
	}
	if req.OutDir != nil {
		s.OutDir = *req.OutDir
	}
	if req.SpeedLimitKB != nil {
		s.SpeedLimitKB = *req.SpeedLimitKB
	}
	if req.MaxParallel != nil {
		s.MaxParallel = *req.MaxParallel
	}
	if req.MaxRetries != nil {
		s.MaxRetries = *req.MaxRetries
	}
	if req.PreferredTheme != nil {
		s.PreferredTheme = *req.PreferredTheme
	}
	if req.FileConflict != nil {
		s.FileConflict = *req.FileConflict
	}
	if req.Proxy != nil {
		s.Proxy = *req.Proxy
	}

	if req.PostDownload.Action != nil {
		s.PostDownload.Action = *req.PostDownload.Action
	}
	if req.PostDownload.AutoOpenDir != nil {
		s.PostDownload.AutoOpenDir = *req.PostDownload.AutoOpenDir
	}

	s.Validate()

	path := filesystem.CreateMetadataFile("settings.json")
	data, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal settings: %w", err)
	}
	if err := filesystem.AtomicWriteFile(path, data, 0o644); err != nil {
		return fmt.Errorf("Write setting: %w", err)
	}

	return nil
}

func (s *Setting) Save() error {
	s.Validate()

	path := filesystem.CreateMetadataFile("settings.json")
	data, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return err
	}

	// Atomic write so a crash during save cannot corrupt settings.json.
	return filesystem.AtomicWriteFile(path, data, 0o644)
}

func (s *Setting) setDefaults() {
	s.StartOnLaunch = true
	s.ConfirmOnExit = true
	s.Silent = false
	s.OutDir = filesystem.GetOrCreateDownloadDirectory()
	s.SpeedLimitKB = 0
	s.MaxParallel = 1
	s.MaxRetries = 3
	s.PreferredTheme = "system"
	s.FileConflict = "rename"
	s.LogLevel = "info"
	s.PostDownload.Action = "none"
	s.PostDownload.AutoOpenDir = false
	s.BandwidthSchedule = nil
	s.Proxy = ""
}

func (s *Setting) applyMissingDefaults() {
	if s.MaxParallel == 0 {
		s.MaxParallel = 1
	}
	if s.OutDir == "" {
		s.OutDir = filesystem.GetOrCreateDownloadDirectory()
	}
	if s.PreferredTheme == "" {
		s.PreferredTheme = "system"
	}
	if s.FileConflict == "" {
		s.FileConflict = "rename"
	}
	if s.LogLevel == "" {
		s.LogLevel = "info"
	}
	if s.PostDownload.Action == "" {
		s.PostDownload.Action = "none"
	}
}
