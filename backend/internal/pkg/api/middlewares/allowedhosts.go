package middlewares

import (
	"net"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
)

// AllowedHosts rejects requests whose Host header is not a loopback host. The
// engine binds 127.0.0.1 only, so the sole legitimate Host values are the
// loopback address (or "localhost") with the bound port.
//
// This is the standard defense against DNS rebinding: a malicious web page can
// re-resolve its OWN domain to 127.0.0.1 and then fetch the local API as a
// same-origin request (so CORS does not catch it), but the browser still sends
// Host: attacker.example — which this rejects. The real desktop frontend always
// talks to the resolved 127.0.0.1:<port> base, so it is unaffected. An empty
// Host (not reachable from a browser) is allowed.
func AllowedHosts() gin.HandlerFunc {
	return func(c *gin.Context) {
		if !isLoopbackHost(c.Request.Host) {
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"error": "forbidden host"})
			return
		}
		c.Next()
	}
}

// isLoopbackHost reports whether a Host header value refers to the loopback
// interface. It tolerates an optional port, IPv6 brackets, and "localhost". An
// empty host is treated as loopback (browsers always send a Host, so this only
// covers non-browser/direct callers).
func isLoopbackHost(host string) bool {
	host = strings.TrimSpace(host)
	if host == "" {
		return true
	}
	h := host
	if hostOnly, _, err := net.SplitHostPort(host); err == nil {
		h = hostOnly
	}
	h = strings.ToLower(strings.Trim(h, "[]"))
	if h == "localhost" {
		return true
	}
	if ip := net.ParseIP(h); ip != nil {
		return ip.IsLoopback()
	}
	return false
}
