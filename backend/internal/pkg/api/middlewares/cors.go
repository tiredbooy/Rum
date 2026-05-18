package middlewares

import (
	"time"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
)

func Cors(allowOrigins []string) gin.HandlerFunc {
	if len(allowOrigins) == 0 {
		allowOrigins = []string{"*"}
	}

	return cors.New(cors.Config{
		AllowOrigins:        allowOrigins,
		AllowMethods:        []string{"GET", "POST", "PUT", "PATCH", "DELETE", "OPTIONS"},
		AllowHeaders:        []string{"Origin", "Content-Type", "Accept", "Authorization", "X-Requested-With"},
		ExposeHeaders:       []string{"Content-Length", "X-Request-ID", "X-Total-Count"},
		AllowCredentials:    false,
		MaxAge:              12 * time.Hour,
		AllowPrivateNetwork: true,
	})
}
