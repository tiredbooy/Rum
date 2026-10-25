package handlers

import (
	"context"
	"errors"
	"log"
	"net/http"
	"runtime"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/tiredbooy/Rum/backend/internal/pkg/api/dto"
	"github.com/tiredbooy/Rum/backend/internal/pkg/config"
	"github.com/tiredbooy/Rum/backend/internal/pkg/download"
	"github.com/tiredbooy/Rum/backend/internal/pkg/updater"
)

// updateService holds the throttle/dismissal state. Current is read from
// AppVersion on every use, so a test (or an ldflags build) can change it.
var updateService = &updater.Service{}

const (
	updateCheckTimeout    = 25 * time.Second
	updateDownloadTimeout = 20 * time.Minute
)

func updateOptions(s config.Setting) updater.Options {
	o := updater.Options{Enabled: s.CheckForUpdates, Proxy: s.Proxy}
	if m := updater.CleanMirrors(s.UpdateMirrors); len(m) > 0 {
		o.Mirrors = m
	}
	return o
}

func currentUpdateService() *updater.Service {
	updateService.Current = AppVersion
	return updateService
}

// GetUpdateStatus returns the last known update result without any network call.
//
//	GET /api/v1/update
func GetUpdateStatus(c *gin.Context) {
	c.JSON(http.StatusOK, currentUpdateService().Status())
}

type updateCheckRequest struct {
	// Auto marks the startup check: it obeys the setting and the once-a-day
	// throttle. Without it the check always runs ("Check now").
	Auto bool `json:"auto"`
}

// CheckUpdate looks for a newer release.
//
//	POST /api/v1/update/check
func CheckUpdate(c *gin.Context) {
	var req updateCheckRequest
	_ = c.ShouldBindJSON(&req) // an empty body means a manual check
	setting, ok := loadSettings(c)
	if !ok {
		return
	}
	ctx, cancel := context.WithTimeout(c.Request.Context(), updateCheckTimeout)
	defer cancel()
	svc := currentUpdateService()
	if req.Auto {
		c.JSON(http.StatusOK, svc.CheckAuto(ctx, updateOptions(setting)))
		return
	}
	c.JSON(http.StatusOK, svc.CheckNow(ctx, updateOptions(setting)))
}

type updateDismissRequest struct {
	Tag string `json:"tag"`
}

// DismissUpdate hides the notice for one release ("Later").
//
//	POST /api/v1/update/dismiss
func DismissUpdate(c *gin.Context) {
	var req updateDismissRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		writeError(c, http.StatusBadRequest, dto.CodeValidation, "invalid request body")
		return
	}
	svc := currentUpdateService()
	if err := svc.Dismiss(req.Tag); err != nil {
		writeFieldErrors(c, "validation failed", map[string]string{"tag": "Not a release tag."})
		return
	}
	c.JSON(http.StatusOK, svc.Status())
}

// DownloadUpdate saves the installer for this system into the save location and
// verifies it against the release's SHA256SUMS. It never installs it.
//
//	POST /api/v1/update/download
func DownloadUpdate(c *gin.Context) {
	setting, ok := loadSettings(c)
	if !ok {
		return
	}
	ctx, cancel := context.WithTimeout(c.Request.Context(), updateDownloadTimeout)
	defer cancel()
	path, err := currentUpdateService().Download(ctx, updateOptions(setting), runtime.GOOS, runtime.GOARCH, setting.OutDir)
	switch {
	case err == nil:
		c.JSON(http.StatusOK, gin.H{"path": path, "verified": true})
	case errors.Is(err, updater.ErrNoAsset):
		writeError(c, http.StatusNotFound, dto.CodeNotFound, "no download for this system")
	case errors.Is(err, updater.ErrChecksumMismatch), errors.Is(err, updater.ErrNoChecksum):
		log.Printf("[request_id=%s] update verify failed: %v", c.GetString("request_id"), err)
		writeError(c, http.StatusUnprocessableEntity, download.ErrCodeUpdateVerify, "download could not be verified and was deleted")
	default:
		log.Printf("[request_id=%s] update download failed: %v", c.GetString("request_id"), err)
		writeError(c, http.StatusBadGateway, download.ErrCodeUpdateCheck, "update server unreachable")
	}
}
