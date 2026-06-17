package handlers

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/tiredbooy/Rum/backend/internal/pkg/api/dto"
	"github.com/tiredbooy/Rum/backend/internal/pkg/config"
)

// GetCategories returns the current auto-organize toggle and category rules.
//
//	GET /api/v1/settings/categories -> { enabled, rules: CategoryRule[] }
func GetCategories(c *gin.Context) {
	var setting config.Setting
	if err := setting.LoadSettingMetadata(); err != nil {
		writeError(c, http.StatusInternalServerError, dto.CodeInternal, "failed to load settings")
		return
	}

	rules := setting.Categories
	if rules == nil {
		rules = []config.CategoryRule{}
	}
	c.JSON(http.StatusOK, dto.CategorySettings{
		Enabled: setting.EnableCategories,
		Rules:   rules,
	})
}

// PutCategories replaces the auto-organize toggle and category rules.
//
//	PUT /api/v1/settings/categories  body: { enabled, rules: CategoryRule[] }
//
// Invalid rules (empty name or no extensions) are rejected at the edge and also
// pruned by config.Setting.Validate on Save.
func PutCategories(c *gin.Context) {
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, maxSettingBodyBytes)

	var req dto.CategorySettings
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

	setting.EnableCategories = req.Enabled
	setting.Categories = req.Rules

	if err := setting.Save(); err != nil {
		writeError(c, http.StatusInternalServerError, dto.CodeInternal, "failed to save categories")
		return
	}

	rules := setting.Categories
	if rules == nil {
		rules = []config.CategoryRule{}
	}
	c.JSON(http.StatusOK, dto.CategorySettings{
		Enabled: setting.EnableCategories,
		Rules:   rules,
	})
}
