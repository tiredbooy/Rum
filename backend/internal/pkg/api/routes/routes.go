package routes

import (
	"github.com/gin-gonic/gin"
	"github.com/tiredbooy/Rum/backend/internal/pkg/api/handlers"
)

func SetupRouter(r *gin.Engine) {
	api := r.Group("/api")
	v1 := api.Group("/v1")
	{
		v1.POST("/downloads", handlers.CreateDownload)
		v1.GET("/downloads", handlers.GetAllJobs)
		v1.DELETE("/downloads", handlers.DeleteDownloads)
		v1.GET("/downloads/stream", handlers.StreamAllProgress)
		v1.GET("/downloads/stats", handlers.GetDownloadsStats)

		v1.POST("/downloads/start-all", handlers.StartDownloads)
		v1.POST("/downloads/pause-all", handlers.PauseDownloads)

		v1.GET("/downloads/:id", handlers.GetDownloadStatus)
		v1.DELETE("/downloads/:id", handlers.DeleteDownload)

		v1.POST("/downloads/:id/start", handlers.StartDownload)
		v1.GET("/downloads/:id/stream", handlers.StreamProgress)
		v1.PUT("/downloads/:id/pause", handlers.PauseDownload)
		v1.PUT("/downloads/:id/resume", handlers.ResumeDownload)
		v1.POST("/downloads/:id/retry", handlers.RetryDownload)
		v1.PATCH("/downloads/:id/priority", handlers.SetDownloadPriority)

		v1.GET("/settings", handlers.GetSettings)
		v1.PATCH("/settings", handlers.UpdateSetting)
		v1.PUT("/settings/speed-limit", handlers.UpdateSpeedLimit)

		v1.GET("/settings/schedule", handlers.GetSchedule)
		v1.PUT("/settings/schedule", handlers.PutSchedule)

		v1.GET("/settings/categories", handlers.GetCategories)
		v1.PUT("/settings/categories", handlers.PutCategories)

		v1.POST("/downloads/batch", handlers.CreateBatch)
	}
}
