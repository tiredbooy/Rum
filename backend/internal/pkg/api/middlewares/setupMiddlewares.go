package middlewares

import (
	"github.com/gin-contrib/gzip"
	"github.com/gin-gonic/gin"
	"golang.org/x/time/rate"
)

func SetupMiddlewares(r *gin.Engine) {
	r.Use(gin.Logger())
	r.Use(gin.Recovery())
	r.Use(rateLimitMiddleware())
	r.Use(Cors([]string{"*"}))
	r.Use(gzip.Gzip(gzip.DefaultCompression))
}

func rateLimitMiddleware() gin.HandlerFunc {
	limiter := rate.NewLimiter(rate.Limit(20), 20)
	return func(c *gin.Context) {
		if !limiter.Allow() {
			c.AbortWithStatusJSON(429, gin.H{"error": "Too many requests"})
			return
		}
		c.Next()
	}
}
