package middlewares

import (
	"net/http"
	"strings"
	"time"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	"github.com/tiredbooy/Rum/backend/internal/pkg/api/dto"
)

// ExtTokenHeader carries the pairing token on every /ext/* request.
const ExtTokenHeader = "X-Rum-Token"

// ExtAppHeader marks every /ext/* response (even a 401) as coming from Rum, so
// the extension can find Rum without sending its token to other local services.
const ExtAppHeader = "X-Rum-App"

// extPathPrefix is the only part of the API browser extensions may reach.
const extPathPrefix = "/ext/"

// Extension rate limit: far below the global one. A paired extension sends a
// request per captured download, so this is generous for people and tight for a
// guessing loop.
const (
	extRPS   = 10
	extBurst = 20
)

func isExtPath(p string) bool { return strings.HasPrefix(p, extPathPrefix) }

// isExtensionOrigin reports whether origin is a WebExtension page. Browsers set
// Origin themselves, so a web page cannot claim to be one.
func isExtensionOrigin(origin string) bool {
	return strings.HasPrefix(origin, "chrome-extension://") ||
		strings.HasPrefix(origin, "moz-extension://")
}

// ExtensionAwareCors wraps the app CORS middleware. Extension origins are
// admitted by a second, narrower CORS policy and ONLY on /ext/*; everywhere
// else (and for every other origin) the original policy applies unchanged, so
// chrome-extension:// and moz-extension:// stay locked out of /api/v1.
func ExtensionAwareCors(base gin.HandlerFunc) gin.HandlerFunc {
	ext := cors.New(cors.Config{
		AllowOriginFunc: isExtensionOrigin,
		AllowMethods:    []string{"GET", "POST", "OPTIONS"},
		AllowHeaders:    []string{"Content-Type", "Accept", ExtTokenHeader},
		ExposeHeaders:   []string{"X-Request-ID", ExtAppHeader},
		MaxAge:          12 * time.Hour,
	})
	return func(c *gin.Context) {
		if isExtPath(c.Request.URL.Path) && isExtensionOrigin(c.Request.Header.Get("Origin")) {
			ext(c)
			return
		}
		base(c)
	}
}

// ExtGuard authenticates /ext/* requests: when an Origin is present it must be
// an extension, and the pairing token must verify. verify is expected to compare
// in constant time (config.VerifyExtensionToken does).
func ExtGuard(verify func(token string) bool) gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Header(ExtAppHeader, "rum")
		if origin := c.GetHeader("Origin"); origin != "" && !isExtensionOrigin(origin) {
			c.AbortWithStatusJSON(http.StatusForbidden, dto.ErrorResponse{
				Error: "origin not allowed", Code: dto.CodeForbidden,
			})
			return
		}
		if !verify(c.GetHeader(ExtTokenHeader)) {
			c.AbortWithStatusJSON(http.StatusUnauthorized, dto.ErrorResponse{
				Error: "pair the extension with Rum", Code: dto.CodeUnauthorized,
			})
			return
		}
		c.Next()
	}
}

// ExtRateLimit is the dedicated limiter for /ext/*. A zero cfg uses the
// extension defaults.
func ExtRateLimit(cfg RateLimitConfig) gin.HandlerFunc {
	if cfg.RPS <= 0 {
		cfg.RPS = extRPS
	}
	if cfg.Burst <= 0 {
		cfg.Burst = extBurst
	}
	return RateLimit(cfg)
}
