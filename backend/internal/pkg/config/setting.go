package config

import (
	"encoding/json"
	"fmt"
	"os"

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
		Action      string `json:"action"` // "none", "shutdown", "sleep"
		AutoOpenDir bool   `json:"auto_open_dir"`
	} `json:"post_download,omitempty"`
	FileConflict string `json:"file_conflict"` // "rename", "overwrite", "skip"
	Proxy        string `json:"proxy,omitempty"`
	LogLevel     string `json:"log_level"` // "info", "debug"
}

type SpeedRule struct {
	StartHour int `json:"start_hour"` // 0-23
	EndHour   int `json:"end_hour"`
	LimitKBps int `json:"limit_kbps"` // 0 = unlimited
}

func (s *Setting) LoadSettingMetadata() error {
	data, err := filesystem.ReadMetadataFile("settings.json")
	if err != nil {
		if os.IsNotExist(err) {
			s.setDefaults()
			return s.Save()
		}
		return fmt.Errorf("Read settings: %w", err)
	}

	if err := json.Unmarshal(data, s); err != nil {
		return fmt.Errorf("Parse settings: %w", err)
	}

	s.applyMissingDefaults()
	return nil

}

func (s *Setting) Save() error {
	path := filesystem.CreateMetadataFile("settings.json")
	data, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(path, data, 0644)
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
