package middlewares

import (
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// RequestIDHeader is the canonical header used to carry the correlation ID on
// both the inbound request and the outbound response.
const RequestIDHeader = "X-Request-ID"

// RequestIDContextKey is the gin context key under which the request ID is
// stored. Handlers read it with c.GetString(RequestIDContextKey).
const RequestIDContextKey = "request_id"

// RequestID attaches a correlation ID to every request.
//
// If the client supplies an X-Request-ID header we honor it (so a trace can be
// followed across a proxy/gateway); otherwise we generate a v4 UUID. The ID is
// stored in the gin context under "request_id" for handlers/loggers and echoed
// back in the response header so a client can correlate a response with any
// server-side log line.
func RequestID() gin.HandlerFunc {
	return func(c *gin.Context) {
		rid := c.GetHeader(RequestIDHeader)
		if rid == "" {
			rid = uuid.NewString()
		}
		c.Set(RequestIDContextKey, rid)
		c.Writer.Header().Set(RequestIDHeader, rid)
		c.Next()
	}
}
