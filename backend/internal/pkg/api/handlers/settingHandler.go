package handlers

import (
	"errors"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/tiredbooy/Rum/backend/internal/pkg/api/dto"
	"github.com/tiredbooy/Rum/backend/internal/pkg/config"
	"github.com/tiredbooy/Rum/backend/internal/pkg/download"
)

const maxSettingBodyBytes = 1 << 20 // 1 MiB

// loadSettings reads the persisted settings for a request handler. A load that
// failed but RECOVERED (config.ErrRecovered — a missing or corrupt file that has
// already been rewritten with defaults) is reported as success with those
// defaults: the settings the caller gets are valid and the file on disk is
// clean, so answering 500 would only make the settings page claim it could not
// load preferences it is in fact holding. Anything else is a genuine failure.
func loadSettings(c *gin.Context) (config.Setting, bool) {
	var setting config.Setting
	err := setting.LoadSettingMetadata()
	if err == nil {
		return setting, true
	}
	if errors.Is(err, config.ErrRecovered) {
		log.Printf("[request_id=%s] settings recovered with defaults: %v", c.GetString("request_id"), err)
		return setting, true
	}
	writeError(c, http.StatusInternalServerError, dto.CodeInternal, "failed to load settings")
	return setting, false
}

func GetSettings(c *gin.Context) {
	setting, ok := loadSettings(c)
	if !ok {
		return
	}

	c.JSON(http.StatusOK, setting)
}

func UpdateSetting(c *gin.Context) {
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, maxSettingBodyBytes)

	var settingReq config.SettingReq
	if err := c.ShouldBindJSON(&settingReq); err != nil {
		writeError(c, http.StatusBadRequest, dto.CodeValidation, "invalid request body")
		return
	}

	if fields := validateSettingReq(settingReq); fields != nil {
		writeFieldErrors(c, "validation failed", fields)
		return
	}

	setting, ok := loadSettings(c)
	if !ok {
		return
	}

	if err := setting.Update(settingReq); err != nil {
		writeError(c, http.StatusInternalServerError, dto.CodeInternal, "failed to update settings")
		return
	}

	applyDownloadOptions(&setting)

	c.JSON(http.StatusOK, setting)
}

// UpdateSpeedLimit is a focused convenience endpoint to set the global download
// speed limit (KB/s, 0 = unlimited) without sending the whole settings object.
//
// It persists the limit via the settings store. NOTE: applying the new limit to
// already-running downloads requires an engine change (see REQUESTED UPSTREAM
// CHANGES); newly started downloads pick up the persisted value.
func UpdateSpeedLimit(c *gin.Context) {
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, maxSettingBodyBytes)

	var req dto.SpeedLimitRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		writeError(c, http.StatusBadRequest, dto.CodeValidation, "invalid request body")
		return
	}
	if fields := req.Validate(); fields != nil {
		writeFieldErrors(c, "validation failed", fields)
		return
	}

	setting, ok := loadSettings(c)
	if !ok {
		return
	}

	if err := setting.Update(config.SettingReq{SpeedLimitKB: req.SpeedLimitKB}); err != nil {
		writeError(c, http.StatusInternalServerError, dto.CodeInternal, "failed to update speed limit")
		return
	}

	applyDownloadOptions(&setting)

	c.JSON(http.StatusOK, gin.H{"speed_limit_kb": setting.SpeedLimitKB})
}

// applyDownloadOptions pushes the freshly-saved settings onto the LIVE download
// engine so a change takes effect on the next download rather than at the next
// app launch. See JobManager.ApplySettings for the full list of what is pushed.
//
// download.LoadOptions is still called for the CLI/TUI code paths that read the
// package-level Opt; it is a one-shot sync.Once, so on the desktop path (where
// cmd/server already built the real Options with the governor and downloader) it
// is a no-op and the manager is the thing that matters.
func applyDownloadOptions(setting *config.Setting) {
	download.LoadOptions(&download.Options{
		SpeedLimit:  setting.SpeedLimitKB,
		Out:         setting.OutDir,
		Parallel:    setting.MaxParallel,
		Connections: setting.Connections,
		Silent:      setting.Silent,
		MaxRetries:  setting.MaxRetries,
		// Reliability / UX settings.
		VerifyIntegrity:      setting.VerifyIntegrity,
		RetryBackoffSec:      setting.RetryBackoffSec,
		TempDir:              setting.TempDir,
		KeepPartialOnFailure: setting.KeepPartialOnFailure,
		Categorize:           setting.EnableCategories,
	})

	if GlobalManager != nil {
		GlobalManager.ApplySettings(*setting)
	}
}

// validateSettingReq returns per-field problems for a PATCH body, so the UI can
// show an inline message next to the offending control. Enum and proxy values
// are rejected here rather than left to config.Validate's silent coercion — a
// typo that quietly became "rename"/"none"/"" looked to the user like a setting
// that refused to save.
func validateSettingReq(req config.SettingReq) map[string]string {
	fields := map[string]string{}
	if req.SpeedLimitKB != nil && *req.SpeedLimitKB < 0 {
		fields["speed_limit_kb"] = "Must be 0 or more."
	}
	if req.MaxParallel != nil && (*req.MaxParallel < 1 || *req.MaxParallel > 64) {
		fields["max_parallel"] = "Must be 1 to 64."
	}
	if req.MaxRetries != nil && (*req.MaxRetries < 0 || *req.MaxRetries > 100) {
		fields["max_retries"] = "Must be 0 to 100."
	}
	if req.Connections != nil && (*req.Connections < 1 || *req.Connections > 16) {
		fields["connections"] = "Must be 1 to 16."
	}
	if req.RetryBackoffSec != nil && (*req.RetryBackoffSec < 1 || *req.RetryBackoffSec > 60) {
		fields["retry_backoff_sec"] = "Must be 1 to 60 seconds."
	}
	if req.PreferredTheme != nil && !config.ValidTheme(*req.PreferredTheme) {
		fields["preferred_theme"] = "Choose System, Light or Dark."
	}
	if req.UIDensity != nil && !config.ValidDensity(*req.UIDensity) {
		fields["ui_density"] = "Choose Comfortable or Compact."
	}
	if req.FileConflict != nil && !config.ValidConflict(*req.FileConflict) {
		fields["file_confilict"] = "Choose Rename, Overwrite or Skip."
	}
	if req.PostDownload.Action != nil && !config.ValidAction(*req.PostDownload.Action) {
		fields["post_download.action"] = "Choose Nothing, Shutdown, Sleep or Close."
	}
	if req.AccentColor != nil && !config.ValidAccentColor(*req.AccentColor) {
		fields["accent_color"] = "Use a hex color like #6366f1."
	}
	if req.LogLevel != nil && !config.ValidLogLevel(*req.LogLevel) {
		fields["log_level"] = "Choose Normal or Debug."
	}
	if req.Proxy != nil {
		if _, ok := config.NormalizeProxy(*req.Proxy); !ok {
			fields["proxy"] = "Use host:port, or an http/https/socks5 URL."
		}
	}
	if req.OutDir != nil {
		if msg := validateWritableDir(*req.OutDir, false); msg != "" {
			fields["out_dir"] = msg
		}
	}
	if req.TempDir != nil {
		if msg := validateWritableDir(*req.TempDir, true); msg != "" {
			fields["temp_dir"] = msg
		}
	}
	if len(fields) == 0 {
		return nil
	}
	return fields
}

// validateWritableDir checks that a directory setting names a usable directory,
// returning a short user-facing message when it does not. An empty value is
// accepted (it means "use the default" for out_dir and "next to the file" for
// temp_dir). The directory is created when missing — the same thing the engine
// would do at download time — so the failure surfaces at save time in the
// settings form rather than as a failed download an hour later.
func validateWritableDir(dir string, allowEmpty bool) string {
	dir = strings.TrimSpace(dir)
	if dir == "" {
		if allowEmpty {
			return ""
		}
		return "" // out_dir "" falls back to the default download folder
	}
	if !filepath.IsAbs(dir) {
		return "Use a full path, starting with /."
	}
	if info, err := os.Stat(dir); err == nil {
		if !info.IsDir() {
			return "That is a file, not a folder."
		}
	} else if os.IsNotExist(err) {
		if mkErr := os.MkdirAll(dir, 0o755); mkErr != nil {
			return "That folder could not be created."
		}
	} else {
		return "That folder could not be read."
	}

	probe, err := os.CreateTemp(dir, ".rum-write-test-*")
	if err != nil {
		return "That folder is not writable."
	}
	name := probe.Name()
	_ = probe.Close()
	_ = os.Remove(name)
	return ""
}
