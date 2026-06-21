package config

import (
	"encoding/json"
	"fmt"
	"os"
	"regexp"
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
	// Connections is the number of parallel connections (segments) per download
	// for range-capable files. More connections beat per-connection CDN throttling
	// (clamped to [1, maxConnections]); 1 = single stream.
	Connections int `json:"connections"`

	// UI / Frontend
	PreferredTheme string `json:"preferred_theme"`

	// Advanced (customizable)
	BandwidthSchedule []SpeedRule `json:"bandwidth_schedule,omitempty"`
	// ScheduledStartEnabled gates whether jobs with a future StartAt are started
	// automatically by the schedule controller. The bandwidth windows in
	// BandwidthSchedule always apply regardless of this flag.
	ScheduledStartEnabled bool `json:"scheduled_start_enabled"`
	PostDownload          struct {
		Action      string `json:"action"` // "none", "shutdown", "sleep", "close"
		AutoOpenDir bool   `json:"auto_open_dir"`
	} `json:"post_download,omitempty"`
	FileConflict string `json:"file_confilict"` // "rename", "overwrite", "skip"
	Proxy        string `json:"proxy,omitempty"`
	LogLevel     string `json:"log_level"` // "info", "debug"

	// Categories drive auto-organize: a finished download whose extension matches
	// a rule is moved into the rule's DestDir. EnableCategories is the master
	// toggle; rules are ignored when it is false.
	EnableCategories bool           `json:"enable_categories"`
	Categories       []CategoryRule `json:"categories,omitempty"`

	// Reliability / integrity. VerifyIntegrity makes every finished download
	// compute + store a full-file hash so it can later be re-verified server-free
	// (the corruption fix — safety is the default, so this is true unless turned
	// off). It is threaded into download.Options.VerifyIntegrity.
	VerifyIntegrity bool `json:"verify_integrity"`

	// BlockPrivateHosts is an opt-in SSRF guard. When true, a download whose host
	// resolves to a loopback / link-local (incl. cloud-metadata 169.254.169.254) /
	// private IP is refused at dial time, re-checked against the resolved IP to
	// defeat DNS rebinding. Off by default so legitimate LAN/NAS downloads work.
	// Ignored when a Proxy is set (the proxy, not us, dials the target).
	BlockPrivateHosts bool `json:"block_private_hosts,omitempty"`

	// Auto-retry / resume policy.
	//   AutoResumeOnReconnect — resume interrupted transfers when connectivity
	//     returns (today this is realized by the retry/backoff policy resuming
	//     rather than failing on transient network errors; full network-event
	//     detection is a follow-up).
	//   AutoResumeOnLaunch — on app launch, re-queue jobs that were running/paused
	//     last session so a restart picks up where it left off.
	//   RetryBackoffSec — base exponential-backoff delay (seconds) for the engine
	//     retry config; clamped to [1, 60].
	AutoResumeOnReconnect bool `json:"auto_resume_on_reconnect"`
	AutoResumeOnLaunch    bool `json:"auto_resume_on_launch"`
	RetryBackoffSec       int  `json:"retry_backoff_sec"`

	// Theme / appearance. These are consumed by the frontend theme provider.
	//   AccentColor — "" = app default, otherwise a #RRGGBB or #RGB hex color.
	//   UIDensity   — "comfortable" | "compact".
	//   ReducedMotion — disable non-essential animations.
	AccentColor   string `json:"accent_color"`
	UIDensity     string `json:"ui_density"`
	ReducedMotion bool   `json:"reduced_motion"`

	// Temp / partial handling.
	//   TempDir — when set, in-progress files are written here and moved to OutDir
	//     on completion; "" = write next to the final output (legacy behavior).
	//   KeepPartialOnFailure — when true, partial data is NOT deleted when a
	//     download fails, so it can be resumed/repaired. (Explicit deletes always
	//     remove partials regardless of this flag.)
	TempDir              string `json:"temp_dir"`
	KeepPartialOnFailure bool   `json:"keep_partial_on_failure"`

	// Desktop / Wails preferences. These live in the backend config (the root
	// module imports it) but are read/written by the desktop layer.
	LaunchOnStartup      bool        `json:"launch_on_startup"` // OS login autostart
	MinimizeToTray       bool        `json:"minimize_to_tray"`  // hide to tray on minimise
	CloseToTray          bool        `json:"close_to_tray"`     // hide to tray instead of quitting
	EnableClipboardWatch bool        `json:"enable_clipboard_watch"`
	WindowState          WindowState `json:"window_state"`
}

// WindowState persists the desktop window geometry so it can be restored on the
// next launch. Zero values mean "no saved state" (the desktop layer then uses
// its defaults).
type WindowState struct {
	W         int  `json:"w"`
	H         int  `json:"h"`
	X         int  `json:"x"`
	Y         int  `json:"y"`
	Maximized bool `json:"maximized"`
}

// CategoryRule maps a set of file extensions to a destination directory. A
// finished download whose extension matches Extensions is moved into DestDir
// (absolute, or relative to the download OutDir).
type CategoryRule struct {
	Name       string   `json:"name"`
	Extensions []string `json:"extensions"` // e.g. [".mp4", ".mkv"]
	DestDir    string   `json:"dest_dir"`   // abs, or relative to OutDir
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
	Connections  *int    `json:"connections"`

	// UI / Frontend
	PreferredTheme *string `json:"preferred_theme"`

	PostDownload struct {
		Action      *string `json:"action"` // "none", "shutdown", "sleep", "close"
		AutoOpenDir *bool   `json:"auto_open_dir"`
	} `json:"post_download,omitempty"`
	FileConflict *string `json:"file_confilict"` // "rename", "overwrite", "skip"
	Proxy        *string `json:"proxy,omitempty"`

	// Desktop / Wails preference toggles (partial-update via PATCH /settings).
	LaunchOnStartup      *bool        `json:"launch_on_startup"`
	MinimizeToTray       *bool        `json:"minimize_to_tray"`
	CloseToTray          *bool        `json:"close_to_tray"`
	EnableClipboardWatch *bool        `json:"enable_clipboard_watch"`
	WindowState          *WindowState `json:"window_state"`

	// Reliability / integrity, auto-retry/resume, theme, temp/partial. All
	// pointers so a PATCH only touches what it sends (partial update semantics).
	VerifyIntegrity       *bool   `json:"verify_integrity"`
	BlockPrivateHosts     *bool   `json:"block_private_hosts,omitempty"`
	AutoResumeOnReconnect *bool   `json:"auto_resume_on_reconnect"`
	AutoResumeOnLaunch    *bool   `json:"auto_resume_on_launch"`
	RetryBackoffSec       *int    `json:"retry_backoff_sec"`
	AccentColor           *string `json:"accent_color"`
	UIDensity             *string `json:"ui_density"`
	ReducedMotion         *bool   `json:"reduced_motion"`
	TempDir               *string `json:"temp_dir"`
	KeepPartialOnFailure  *bool   `json:"keep_partial_on_failure"`
}

type SpeedRule struct {
	StartHour int   `json:"start_hour"`     // 0-23
	EndHour   int   `json:"end_hour"`       // 0-23
	LimitKBps int   `json:"limit_kbps"`     // 0 = unlimited
	Days      []int `json:"days,omitempty"` // 0=Sun..6=Sat; empty = every day
}

const (
	// defaultConnections is the default parallel connections per download. 8
	// mirrors the engine default and IDM; maxConnections bounds the setting/slider.
	defaultConnections = 8
	maxConnections     = 16

	// defaultRetryBackoffSec is the default base exponential-backoff delay in
	// seconds, mirroring the engine's defaultRetryBaseDelay (500ms rounds up to a
	// 1s base here, the minimum of the [1,60] clamp). Threaded into the engine
	// retry config.
	defaultRetryBackoffSec = 1
	minRetryBackoffSec     = 1
	maxRetryBackoffSec     = 60
)

// validActions / validConflicts / validLogLevels / validDensities define the
// accepted enum values.
var (
	validActions    = map[string]bool{"none": true, "shutdown": true, "sleep": true, "close": true}
	validConflicts  = map[string]bool{"rename": true, "overwrite": true, "skip": true}
	validLogLevels  = map[string]bool{"info": true, "debug": true, "warn": true, "error": true}
	validDensities  = map[string]bool{"comfortable": true, "compact": true}
	hexColorPattern = regexp.MustCompile(`^#(?:[0-9a-fA-F]{3}|[0-9a-fA-F]{6})$`)
)

// normalizeAccentColor returns the input trimmed if it is a valid #RGB / #RRGGBB
// hex color, otherwise "" (the app-default sentinel). Empty input is valid and
// stays "". This keeps an invalid color from ever reaching the theme provider.
func normalizeAccentColor(c string) string {
	c = strings.TrimSpace(c)
	if c == "" {
		return ""
	}
	if hexColorPattern.MatchString(c) {
		return c
	}
	return "" // reject invalid -> app default
}

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
	if s.Connections < 1 {
		s.Connections = defaultConnections
	}
	if s.Connections > maxConnections {
		s.Connections = maxConnections
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
	// Clamp bandwidth schedule hours/limits and prune invalid day-of-week entries.
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
		// Drop any day index outside 0..6 (Sun..Sat). An empty/nil Days slice means
		// "every day", which is preserved.
		if len(r.Days) > 0 {
			valid := r.Days[:0]
			for _, d := range r.Days {
				if d >= 0 && d <= 6 {
					valid = append(valid, d)
				}
			}
			r.Days = valid
		}
	}

	// Drop category rules that can never match (no name, or no extensions). An
	// empty DestDir is allowed and means "leave next to the download / OutDir".
	if len(s.Categories) > 0 {
		valid := s.Categories[:0]
		for _, c := range s.Categories {
			if strings.TrimSpace(c.Name) == "" || len(c.Extensions) == 0 {
				continue
			}
			valid = append(valid, c)
		}
		s.Categories = valid
	}

	// Retry backoff base: clamp to [1, 60] seconds (0/negative means "use the
	// default" rather than an impossible zero-delay retry storm).
	if s.RetryBackoffSec <= 0 {
		s.RetryBackoffSec = defaultRetryBackoffSec
	}
	if s.RetryBackoffSec < minRetryBackoffSec {
		s.RetryBackoffSec = minRetryBackoffSec
	}
	if s.RetryBackoffSec > maxRetryBackoffSec {
		s.RetryBackoffSec = maxRetryBackoffSec
	}

	// Theme/appearance: reject an invalid accent color (-> app default) and an
	// unknown density (-> comfortable). TempDir is trimmed but otherwise free-form
	// (path validity is checked at use-time when the dir is created).
	s.AccentColor = normalizeAccentColor(s.AccentColor)
	if !validDensities[s.UIDensity] {
		s.UIDensity = "comfortable"
	}
	s.TempDir = strings.TrimSpace(s.TempDir)
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
	if req.Connections != nil {
		s.Connections = *req.Connections
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

	// Desktop preference toggles.
	if req.LaunchOnStartup != nil {
		s.LaunchOnStartup = *req.LaunchOnStartup
	}
	if req.MinimizeToTray != nil {
		s.MinimizeToTray = *req.MinimizeToTray
	}
	if req.CloseToTray != nil {
		s.CloseToTray = *req.CloseToTray
	}
	if req.EnableClipboardWatch != nil {
		s.EnableClipboardWatch = *req.EnableClipboardWatch
	}
	if req.WindowState != nil {
		s.WindowState = *req.WindowState
	}

	// Reliability / integrity, auto-retry/resume, theme, temp/partial.
	if req.VerifyIntegrity != nil {
		s.VerifyIntegrity = *req.VerifyIntegrity
	}
	if req.BlockPrivateHosts != nil {
		s.BlockPrivateHosts = *req.BlockPrivateHosts
	}
	if req.AutoResumeOnReconnect != nil {
		s.AutoResumeOnReconnect = *req.AutoResumeOnReconnect
	}
	if req.AutoResumeOnLaunch != nil {
		s.AutoResumeOnLaunch = *req.AutoResumeOnLaunch
	}
	if req.RetryBackoffSec != nil {
		s.RetryBackoffSec = *req.RetryBackoffSec
	}
	if req.AccentColor != nil {
		s.AccentColor = *req.AccentColor
	}
	if req.UIDensity != nil {
		s.UIDensity = *req.UIDensity
	}
	if req.ReducedMotion != nil {
		s.ReducedMotion = *req.ReducedMotion
	}
	if req.TempDir != nil {
		s.TempDir = *req.TempDir
	}
	if req.KeepPartialOnFailure != nil {
		s.KeepPartialOnFailure = *req.KeepPartialOnFailure
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

// DisarmDestructivePostDownload resets a persisted shutdown/sleep/close action
// to "none" so it can never auto-fire after a restart — the user must re-arm it
// in the current session. Called once at startup. Persisting a destructive power
// action silently means a download finishing right after launch could power off
// the machine unexpectedly (and a settings write could weaponize it). The
// harmless AutoOpenDir flag is left untouched. Returns true if it changed.
func (s *Setting) DisarmDestructivePostDownload() bool {
	switch s.PostDownload.Action {
	case "shutdown", "sleep", "close":
		s.PostDownload.Action = "none"
		return true
	default:
		return false
	}
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
	s.Connections = defaultConnections
	s.MaxRetries = 3
	s.PreferredTheme = "system"
	s.FileConflict = "rename"
	s.LogLevel = "info"
	s.PostDownload.Action = "none"
	s.PostDownload.AutoOpenDir = false
	s.BandwidthSchedule = nil
	s.ScheduledStartEnabled = false
	s.Proxy = ""

	// Auto-organize: off by default with no rules.
	s.EnableCategories = false
	s.Categories = nil

	// Desktop preferences: all opt-in (off) by default; no saved window geometry.
	s.LaunchOnStartup = false
	s.MinimizeToTray = false
	s.CloseToTray = false
	s.EnableClipboardWatch = false
	s.WindowState = WindowState{}

	// Reliability / UX defaults. Safety-first: integrity verification and
	// auto-resume are ON by default (this is the corruption fix). Theme/temp
	// settings default to the app's existing look + "next to output" behavior.
	s.VerifyIntegrity = true
	s.BlockPrivateHosts = false
	s.AutoResumeOnReconnect = true
	s.AutoResumeOnLaunch = true
	s.RetryBackoffSec = defaultRetryBackoffSec
	s.AccentColor = ""
	s.UIDensity = "comfortable"
	s.ReducedMotion = false
	s.TempDir = ""
	s.KeepPartialOnFailure = true
}

func (s *Setting) applyMissingDefaults() {
	if s.MaxParallel == 0 {
		s.MaxParallel = 1
	}
	if s.Connections == 0 {
		s.Connections = defaultConnections
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

	// Reliability / UX backfill for configs written before these fields existed.
	// Only zero-valued string/int fields are backfilled here: a JSON-missing field
	// unmarshals to the zero value, which is indistinguishable from an explicit
	// zero for bools, so the default-true bools (VerifyIntegrity, AutoResume*,
	// KeepPartialOnFailure) are intentionally NOT forced on here — doing so would
	// re-enable a setting a user deliberately turned off. New installs get the
	// safe defaults via setDefaults(); upgraders can opt in from the settings UI.
	if s.RetryBackoffSec == 0 {
		s.RetryBackoffSec = defaultRetryBackoffSec
	}
	if s.UIDensity == "" {
		s.UIDensity = "comfortable"
	}
}
