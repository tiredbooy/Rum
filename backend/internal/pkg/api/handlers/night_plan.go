package handlers

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/tiredbooy/Rum/backend/internal/pkg/api/dto"
	"github.com/tiredbooy/Rum/backend/internal/pkg/config"
)

// nightPlanResponse pairs a saved plan with the engine's live window state.
func nightPlanResponse(plan config.NightPlan) dto.NightPlanResponse {
	resp := dto.NightPlanResponse{NightPlan: plan}
	if GlobalManager != nil {
		st := GlobalManager.NightStatus(time.Now())
		resp.Active, resp.CapReached, resp.NextStart = st.Active, st.CapReached, st.NextStart
	}
	return resp
}

// GetNightPlan returns the night plan and whether its window is open now.
//
//	GET /api/v1/settings/night-plan
func GetNightPlan(c *gin.Context) {
	setting, ok := loadSettings(c)
	if !ok {
		return
	}
	c.JSON(http.StatusOK, nightPlanResponse(setting.NightPlan))
}

// PutNightPlan replaces the night plan. Invalid times, an empty window
// (start == end) or a bad cap are rejected with field errors, never coerced.
//
//	PUT /api/v1/settings/night-plan  body: { enabled, start, end, start_at_open, pause_at_close, cap_gb }
func PutNightPlan(c *gin.Context) {
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, maxSettingBodyBytes)

	var req config.NightPlan
	if err := c.ShouldBindJSON(&req); err != nil {
		writeError(c, http.StatusBadRequest, dto.CodeValidation, "invalid request body")
		return
	}
	if fields := req.Problems(); fields != nil {
		writeFieldErrors(c, "validation failed", fields)
		return
	}

	settingsWriteMu.Lock()
	defer settingsWriteMu.Unlock()

	setting, ok := loadSettings(c)
	if !ok {
		return
	}
	setting.NightPlan = req
	if err := setting.Save(); err != nil {
		writeError(c, http.StatusInternalServerError, dto.CodeInternal, "failed to save night plan")
		return
	}
	applyDownloadOptions(&setting) // the engine reads the plan from its live settings
	c.JSON(http.StatusOK, nightPlanResponse(setting.NightPlan))
}

// MoveToNightQueue moves downloads into or out of the night queue.
//
//	POST /api/v1/downloads/night-queue  body: { ids: string[], enabled: bool } -> { updated: n }
func MoveToNightQueue(c *gin.Context) {
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, maxSettingBodyBytes)

	var req dto.NightQueueRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		writeError(c, http.StatusBadRequest, dto.CodeValidation, "invalid request body")
		return
	}
	if fields := req.Validate(); fields != nil {
		writeFieldErrors(c, "validation failed", fields)
		return
	}
	updated := GlobalManager.SetNightQueue(req.IDs, req.Enabled, time.Now())
	c.JSON(http.StatusOK, gin.H{"updated": updated})
}
