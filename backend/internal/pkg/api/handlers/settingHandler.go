package handlers

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/tiredbooy/Rum/backend/internal/pkg/config"
)

func GetSettings(c *gin.Context) {
	var setting config.Setting

	err := setting.LoadSettingMetadata()
	if err != nil {
		c.JSON(http.StatusInternalServerError, "Failed to get setting")
		return
	}

	c.JSON(http.StatusOK, setting)
}

func UpdateSetting(c *gin.Context) {
	var settingReq config.SettingReq

	err := c.ShouldBindJSON(&settingReq)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Failed to parse request."})
		return
	}

	var setting config.Setting

	err = setting.Update(settingReq)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update setting"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Setting Updated."})
}
