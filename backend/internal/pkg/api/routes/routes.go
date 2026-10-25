package routes

import (
	"github.com/gin-gonic/gin"
	"github.com/tiredbooy/Rum/backend/internal/pkg/api/handlers"
	"github.com/tiredbooy/Rum/backend/internal/pkg/api/middlewares"
	"github.com/tiredbooy/Rum/backend/internal/pkg/config"
)

func SetupRouter(r *gin.Engine) {
	api := r.Group("/api")
	v1 := api.Group("/v1")
	{
		v1.GET("/status", handlers.GetStatus)

		v1.POST("/downloads", handlers.CreateDownload)
		v1.GET("/downloads", handlers.GetAllJobs)
		v1.DELETE("/downloads", handlers.DeleteDownloads)
		v1.POST("/downloads/restore", handlers.RestoreDownloads)
		v1.GET("/downloads/stream", handlers.StreamAllProgress)
		v1.GET("/downloads/stats", handlers.GetDownloadsStats)
		v1.GET("/downloads/export", handlers.ExportDownloads)
		v1.POST("/downloads/import", handlers.ImportDownloads)
		v1.GET("/diagnostics", handlers.GetDiagnostics)

		v1.POST("/downloads/start-all", handlers.StartDownloads)
		v1.POST("/downloads/pause-all", handlers.PauseDownloads)
		v1.POST("/downloads/retry-failed", handlers.RetryFailedDownloads)

		v1.GET("/downloads/:id", handlers.GetDownloadStatus)
		v1.DELETE("/downloads/:id", handlers.DeleteDownload)

		v1.POST("/downloads/:id/start", handlers.StartDownload)
		v1.POST("/downloads/:id/restore", handlers.RestoreDownload)
		v1.GET("/downloads/:id/stream", handlers.StreamProgress)
		v1.PUT("/downloads/:id/pause", handlers.PauseDownload)
		v1.PUT("/downloads/:id/resume", handlers.ResumeDownload)
		v1.POST("/downloads/:id/retry", handlers.RetryDownload)
		v1.POST("/downloads/:id/download-anyway", handlers.DownloadAnyway)
		v1.POST("/downloads/:id/verify", handlers.VerifyDownload)
		v1.POST("/downloads/:id/verify-checksum", handlers.VerifyDownloadChecksum)
		v1.POST("/downloads/:id/redownload", handlers.RedownloadDownload)
		v1.POST("/downloads/:id/repair", handlers.RepairDownload)
		v1.PATCH("/downloads/:id/priority", handlers.SetDownloadPriority)
		v1.PATCH("/downloads/:id/url", handlers.RefreshDownloadURL)

		v1.GET("/settings", handlers.GetSettings)
		v1.PATCH("/settings", handlers.UpdateSetting)
		v1.PUT("/settings/speed-limit", handlers.UpdateSpeedLimit)

		v1.GET("/settings/schedule", handlers.GetSchedule)
		v1.PUT("/settings/schedule", handlers.PutSchedule)

		v1.GET("/settings/night-plan", handlers.GetNightPlan)
		v1.PUT("/settings/night-plan", handlers.PutNightPlan)
		v1.POST("/downloads/night-queue", handlers.MoveToNightQueue)

		v1.GET("/settings/categories", handlers.GetCategories)
		v1.PUT("/settings/categories", handlers.PutCategories)

		v1.POST("/downloads/batch", handlers.CreateBatch)

		v1.GET("/finish-action", handlers.GetFinishAction)
		v1.POST("/finish-action/cancel", handlers.CancelFinishAction)

		// Pairing token shown in Settings (not reachable by the extension).
		v1.GET("/extension/token", handlers.GetExtensionToken)
		v1.POST("/extension/token/regenerate", handlers.RegenerateExtensionToken)
	}

	// Browser-extension surface: the only routes extension origins may reach.
	// Order matters: rate limit first (so guessing is throttled), then the
	// origin + token check.
	ext := r.Group("/ext", middlewares.ExtRateLimit(middlewares.RateLimitConfig{}), middlewares.ExtGuard(config.VerifyExtensionToken))
	{
		ext.GET("/ping", handlers.ExtPing)
		ext.POST("/add", handlers.ExtAdd)
		ext.POST("/open", handlers.ExtOpen)
	}
}
