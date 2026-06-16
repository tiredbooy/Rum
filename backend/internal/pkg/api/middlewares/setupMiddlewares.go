package middlewares

import (
	"github.com/gin-contrib/gzip"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"golang.org/x/time/rate"
)

// SetupMiddlewares wires the cross-cutting middleware chain in a deliberate
// order (go-api-design): recovery outermost so a panic anywhere downstream is
// turned into a 500 instead of crashing the process, then request-ID for
// correlation, logging, rate limiting, CORS and compression.
func SetupMiddlewares(r *gin.Engine) {
	r.Use(gin.Recovery())     // outermost: catch panics from every later layer
	r.Use(requestIDMiddleware())
	r.Use(gin.Logger())
	r.Use(rateLimitMiddleware())
	r.Use(Cors(nil)) // nil -> env allowlist or localhost dev defaults
	r.Use(gzip.Gzip(gzip.DefaultCompression))
}

// requestIDMiddleware attaches a request ID to every request (honoring an
// inbound X-Request-ID) and echoes it back so errors logged server-side can be
// correlated with the response the client received.
func requestIDMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		rid := c.GetHeader("X-Request-ID")
		if rid == "" {
			rid = uuid.NewString()
		}
		c.Set("request_id", rid)
		c.Writer.Header().Set("X-Request-ID", rid)
		c.Next()
	}
}

func rateLimitMiddleware() gin.HandlerFunc {
	limiter := rate.NewLimiter(rate.Limit(20), 20)
	return func(c *gin.Context) {
		if !limiter.Allow() {
			c.AbortWithStatusJSON(429, gin.H{
				"error": "too many requests",
				"code":  "rate_limited",
			})
			return
		}
		c.Next()
	}
}
