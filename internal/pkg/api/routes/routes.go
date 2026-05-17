package routes

import (
	"github.com/gin-gonic/gin"
	"swiftget.com/internal/pkg/api/handlers"
)

func SetupRouter(r *gin.Engine) {
	api := r.Group("/api")
	v1 := api.Group("/v1")
	{
		v1.POST("/downloads", handlers.CreateDownload)
		v1.GET("/downloads/:id", handlers.GetDownloadStatus)
		// Optional: pause, resume, delete
		// v1.PUT("/downloads/:id/pause", handlers.PauseDownload)
		// v1.PUT("/downloads/:id/resume", handlers.ResumeDownload)
	}
}
