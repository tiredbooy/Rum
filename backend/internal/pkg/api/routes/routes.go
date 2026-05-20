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
		v1.POST("/downloads/start-all", handlers.StartDownloads)
		v1.GET("/downloads", handlers.GetAllJobs)
		v1.GET("/downloads/:id", handlers.GetDownloadStatus)
		v1.DELETE("/downloads/:id", handlers.DeleteDownload)
		v1.POST("/downloads/:id/start", handlers.StartDownload)
		v1.GET("/downloads/:id/stream", handlers.StreamProgress)

		v1.PUT("/downloads/:id/pause", handlers.PauseDownload)
		v1.PUT("/downloads/:id/resume", handlers.ResumeDownload)

		v1.GET("/settings", handlers.GetSettings)
		v1.PATCH("/settings", handlers.UpdateSetting)
	}
}
