package middlewares

import (
	"github.com/gin-contrib/gzip"
	"github.com/gin-gonic/gin"
)

// SetupMiddlewares wires the cross-cutting middleware chain in a deliberate
// order (go-api-design):
//
//	Recovery (outermost) -> RequestID -> Logger -> CORS -> RateLimit -> handler
//
// Recovery is outermost so a panic anywhere downstream becomes a sanitized 500
// instead of crashing the process. RequestID runs early so recovery, the logger
// and every handler share the same correlation ID. CORS runs before the rate
// limiter so that even a rejected request still carries the right CORS headers
// (and so preflight OPTIONS aren't charged a token). gzip compression is last.
func SetupMiddlewares(r *gin.Engine) {
	r.Use(Recovery())   // outermost: catch panics from every later layer
	r.Use(RequestID())  // correlation ID for logs + response header
	r.Use(gin.Logger()) // request logging
	r.Use(Cors(nil))    // nil -> env allowlist or localhost dev defaults
	r.Use(RateLimit(RateLimitConfig{}))
	r.Use(gzip.Gzip(gzip.DefaultCompression))
}
