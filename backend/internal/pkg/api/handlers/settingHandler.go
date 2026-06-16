package handlers

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/tiredbooy/Rum/backend/internal/pkg/api/dto"
	"github.com/tiredbooy/Rum/backend/internal/pkg/config"
	"github.com/tiredbooy/Rum/backend/internal/pkg/download"
)

const maxSettingBodyBytes = 1 << 20 // 1 MiB

func GetSettings(c *gin.Context) {
	var setting config.Setting

	if err := setting.LoadSettingMetadata(); err != nil {
		writeError(c, http.StatusInternalServerError, dto.CodeInternal, "failed to load settings")
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

	var setting config.Setting
	if err := setting.LoadSettingMetadata(); err != nil {
		writeError(c, http.StatusInternalServerError, dto.CodeInternal, "failed to load settings")
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

	var setting config.Setting
	if err := setting.LoadSettingMetadata(); err != nil {
		writeError(c, http.StatusInternalServerError, dto.CodeInternal, "failed to load settings")
		return
	}

	if err := setting.Update(config.SettingReq{SpeedLimitKB: req.SpeedLimitKB}); err != nil {
		writeError(c, http.StatusInternalServerError, dto.CodeInternal, "failed to update speed limit")
		return
	}

	applyDownloadOptions(&setting)

	c.JSON(http.StatusOK, gin.H{"speed_limit_kb": setting.SpeedLimitKB})
}

func applyDownloadOptions(setting *config.Setting) {
	opt := &download.Options{
		SpeedLimit: setting.SpeedLimitKB,
		Out:        setting.OutDir,
		Parallel:   setting.MaxParallel,
		Silent:     setting.Silent,
		MaxRetries: setting.MaxRetries,
	}
	download.LoadOptions(opt)
}

func validateSettingReq(req config.SettingReq) map[string]string {
	fields := map[string]string{}
	if req.SpeedLimitKB != nil && *req.SpeedLimitKB < 0 {
		fields["speed_limit_kb"] = "must be >= 0 (0 means unlimited)"
	}
	if req.MaxParallel != nil && *req.MaxParallel < 1 {
		fields["max_parallel"] = "must be >= 1"
	}
	if req.MaxRetries != nil && *req.MaxRetries < 0 {
		fields["max_retries"] = "must be >= 0"
	}
	if len(fields) == 0 {
		return nil
	}
	return fields
}
