package handlers

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/tiredbooy/Rum/backend/internal/pkg/config"
)

type SettingRes struct {
	ConfirmOnExit  bool   `json:"confirm_on_exit"`
	Silent         bool   `json:"silent"`
	PreferredTheme string `json:"preferred_theme"`
}

func GetSettings(c *gin.Context) {
	var setting config.Setting

	err := setting.LoadSettingMetadata()
	if err != nil {
		c.JSON(http.StatusInternalServerError, "Failed to get setting")
		return
	}

	c.JSON(http.StatusOK, &SettingRes{
		ConfirmOnExit:  setting.ConfirmOnExit,
		Silent:         setting.Silent,
		PreferredTheme: setting.PreferredTheme,
	})
}
