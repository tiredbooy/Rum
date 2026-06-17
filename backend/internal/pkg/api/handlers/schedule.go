package handlers

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/tiredbooy/Rum/backend/internal/pkg/api/dto"
	"github.com/tiredbooy/Rum/backend/internal/pkg/config"
)

// GetSchedule returns the current bandwidth schedule and scheduled-start toggle.
//
//	GET /api/v1/settings/schedule -> { scheduled_start_enabled, rules: SpeedRule[] }
func GetSchedule(c *gin.Context) {
	var setting config.Setting
	if err := setting.LoadSettingMetadata(); err != nil {
		writeError(c, http.StatusInternalServerError, dto.CodeInternal, "failed to load settings")
		return
	}

	rules := setting.BandwidthSchedule
	if rules == nil {
		rules = []config.SpeedRule{}
	}
	c.JSON(http.StatusOK, dto.ScheduleSettings{
		ScheduledStartEnabled: setting.ScheduledStartEnabled,
		Rules:                 rules,
	})
}

// PutSchedule replaces the bandwidth schedule + scheduled-start toggle.
//
//	PUT /api/v1/settings/schedule  body: { scheduled_start_enabled, rules: SpeedRule[] }
//
// Rules are validated/clamped by config.Setting.Validate (hours to 0..23, days to
// 0..6, negative limits to 0) on Save.
func PutSchedule(c *gin.Context) {
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, maxSettingBodyBytes)

	var req dto.ScheduleSettings
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

	setting.ScheduledStartEnabled = req.ScheduledStartEnabled
	setting.BandwidthSchedule = req.Rules

	if err := setting.Save(); err != nil {
		writeError(c, http.StatusInternalServerError, dto.CodeInternal, "failed to save schedule")
		return
	}

	rules := setting.BandwidthSchedule
	if rules == nil {
		rules = []config.SpeedRule{}
	}
	c.JSON(http.StatusOK, dto.ScheduleSettings{
		ScheduledStartEnabled: setting.ScheduledStartEnabled,
		Rules:                 rules,
	})
}
