package middlewares

import (
	"os"
	"strings"
	"time"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
)

// defaultDevOrigins keeps local development (Vite, Tauri, Electron) working out
// of the box without resorting to a wildcard origin.
var defaultDevOrigins = []string{
	"http://localhost:5173",
	"http://127.0.0.1:5173",
	"http://localhost:1420", // tauri dev
	"http://localhost:3000",
	"tauri://localhost",
}

// Cors builds the CORS middleware from an explicit allowlist.
//
// Precedence: explicit allowOrigins arg -> RUM_CORS_ORIGINS env (comma list) ->
// safe localhost dev defaults. We intentionally avoid a wildcard "*": a wildcard
// origin combined with any future credentialed request is a known foot-gun, and
// even without credentials it needlessly widens the attack surface.
func Cors(allowOrigins []string) gin.HandlerFunc {
	origins := resolveOrigins(allowOrigins)

	return cors.New(cors.Config{
		AllowOrigins:        origins,
		AllowMethods:        []string{"GET", "POST", "PUT", "PATCH", "DELETE", "OPTIONS"},
		AllowHeaders:        []string{"Origin", "Content-Type", "Accept", "Authorization", "X-Requested-With", "X-Request-ID"},
		ExposeHeaders:       []string{"Content-Length", "X-Request-ID", "X-Total-Count"},
		AllowCredentials:    false,
		MaxAge:              12 * time.Hour,
		AllowPrivateNetwork: true,
	})
}

func resolveOrigins(allowOrigins []string) []string {
	// Drop a caller-supplied wildcard so it can't silently re-open everything.
	allowOrigins = filterWildcard(allowOrigins)
	if len(allowOrigins) > 0 {
		return allowOrigins
	}

	if env := os.Getenv("RUM_CORS_ORIGINS"); env != "" {
		var out []string
		for _, o := range strings.Split(env, ",") {
			if o = strings.TrimSpace(o); o != "" && o != "*" {
				out = append(out, o)
			}
		}
		if len(out) > 0 {
			return out
		}
	}

	return defaultDevOrigins
}

func filterWildcard(origins []string) []string {
	out := origins[:0:0]
	for _, o := range origins {
		if strings.TrimSpace(o) == "*" || strings.TrimSpace(o) == "" {
			continue
		}
		out = append(out, o)
	}
	return out
}
