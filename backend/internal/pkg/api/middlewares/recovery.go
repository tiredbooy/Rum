package middlewares

import (
	"log"
	"net/http"
	"runtime/debug"

	"github.com/gin-gonic/gin"
	"github.com/tiredbooy/Rum/backend/internal/pkg/api/dto"
)

// Recovery converts any panic from a downstream handler/middleware into a
// sanitized 500 instead of crashing the process or leaking a stack trace to the
// client. The panic value and stack are logged server-side, tagged with the
// request ID, so the incident is debuggable without exposing internals.
//
// This is the outermost middleware so it covers every later layer.
func Recovery() gin.HandlerFunc {
	return func(c *gin.Context) {
		defer func() {
			if r := recover(); r != nil {
				rid := c.GetString(RequestIDContextKey)
				log.Printf("[request_id=%s] panic recovered: %v\n%s", rid, r, debug.Stack())

				// AbortWithStatusJSON is a no-op if the response was already
				// written, so a panic mid-stream won't double-write.
				c.AbortWithStatusJSON(http.StatusInternalServerError, dto.ErrorResponse{
					Error: "internal server error",
					Code:  dto.CodeInternal,
				})
			}
		}()
		c.Next()
	}
}
