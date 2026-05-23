package handlers

import (
	"log"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/tiredbooy/Rum/backend/internal/pkg/config"
	"github.com/tiredbooy/Rum/backend/internal/pkg/download"
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

    if err := c.ShouldBindJSON(&settingReq); err != nil {
        c.JSON(http.StatusBadRequest, gin.H{"error": "Failed to parse request."})
        return
    }

    var setting config.Setting
    if err := setting.LoadSettingMetadata(); err != nil {
        c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to load settings"})
        return
    }

    if err := setting.Update(settingReq); err != nil {
        c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update setting"})
        return
    }

    opt := &download.Options{
        SpeedLimit: setting.SpeedLimitKB,
        Out: setting.OutDir,
        Parallel: setting.MaxParallel,
        Silent: setting.Silent,
        MaxRetries: setting.MaxRetries,
    }

    log.Println("NEW OPT: ", opt)

    download.LoadOptions(opt)

    c.JSON(http.StatusOK, setting)
}