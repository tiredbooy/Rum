package middlewares

import (
	"os"
	"runtime"
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

// desktopWebviewOrigins are the origins the bundled Wails webview loads the
// frontend from before it fetches the loopback API cross-origin. They are ALWAYS
// allowed (regardless of RUM_CORS_ORIGINS or an explicit allowlist) — otherwise
// the app's own UI is blocked by CORS: the OPTIONS preflight and the live SSE
// stream return 403 and the window shows "Connection lost."
var desktopWebviewOrigins = desktopOrigins()

// desktopOrigins returns the webview origins to always allow, scoped to the
// current OS so the allowlist stays as tight as possible:
//
//	Linux/macOS: wails://wails and wails://wails.localhost — a CUSTOM URI scheme
//	  the internal frontend serves from ("wails://wails/"). A normal browser cannot
//	  forge a custom-scheme Origin, so allowing these can't widen the loopback API
//	  to web pages.
//	Windows: additionally http://wails.localhost — WebView2's origin. This is a
//	  plain-http origin a real browser CAN emit (wails.localhost resolves to
//	  127.0.0.1), so it is allowed ONLY on Windows, where the app genuinely needs
//	  it, and never on Linux/macOS where it would needlessly broaden CORS to a
//	  forgeable origin. (CORS is not the security boundary here regardless — the
//	  loopback API has no auth, so keep the surface minimal.)
func desktopOrigins() []string {
	o := []string{"wails://wails", "wails://wails.localhost"}
	if runtime.GOOS == "windows" {
		o = append(o, "http://wails.localhost")
	}
	return o
}

// Cors builds the CORS middleware from an explicit allowlist.
//
// Precedence: explicit allowOrigins arg -> RUM_CORS_ORIGINS env (comma list) ->
// safe localhost dev defaults. We intentionally avoid a wildcard "*": a wildcard
// origin combined with any future credentialed request is a known foot-gun, and
// even without credentials it needlessly widens the attack surface.
func Cors(allowOrigins []string) gin.HandlerFunc {
	origins := resolveOrigins(allowOrigins)

	// gin-contrib/cors panics on any AllowOrigins entry that is neither "*" nor
	// an http(s):// URL. Desktop webview origins use custom schemes (tauri://,
	// wails://), so keep only http(s)/wildcard entries in AllowOrigins and match
	// custom-scheme origins via AllowOriginFunc instead.
	var httpOrigins []string
	customAllowed := make(map[string]bool)
	for _, o := range origins {
		if o == "*" || strings.HasPrefix(o, "http://") || strings.HasPrefix(o, "https://") {
			httpOrigins = append(httpOrigins, o)
		} else {
			customAllowed[o] = true
		}
	}

	cfg := cors.Config{
		AllowMethods:        []string{"GET", "POST", "PUT", "PATCH", "DELETE", "OPTIONS"},
		AllowHeaders:        []string{"Origin", "Content-Type", "Accept", "Authorization", "X-Requested-With", "X-Request-ID"},
		ExposeHeaders:       []string{"Content-Length", "X-Request-ID", "X-Total-Count"},
		AllowCredentials:    false,
		MaxAge:              12 * time.Hour,
		AllowPrivateNetwork: true,
		AllowOrigins:        httpOrigins,
	}
	if len(customAllowed) > 0 {
		cfg.AllowOriginFunc = func(origin string) bool { return customAllowed[origin] }
	}
	// cors.New errors if neither AllowOrigins nor AllowOriginFunc is set.
	if len(cfg.AllowOrigins) == 0 && cfg.AllowOriginFunc == nil {
		cfg.AllowOrigins = []string{"http://localhost"}
	}

	return cors.New(cfg)
}

func resolveOrigins(allowOrigins []string) []string {
	// The app's own webview origins are always allowed, then merged with whatever
	// the caller/env/dev-defaults add, so configuring CORS can never lock the
	// desktop UI out of its own loopback API.
	out := append([]string{}, desktopWebviewOrigins...)

	// Drop a caller-supplied wildcard so it can't silently re-open everything.
	allowOrigins = filterWildcard(allowOrigins)
	if len(allowOrigins) > 0 {
		return append(out, allowOrigins...)
	}

	if env := os.Getenv("RUM_CORS_ORIGINS"); env != "" {
		for _, o := range strings.Split(env, ",") {
			if o = strings.TrimSpace(o); o != "" && o != "*" {
				out = append(out, o)
			}
		}
		if len(out) > len(desktopWebviewOrigins) {
			return out
		}
	}

	return append(out, defaultDevOrigins...)
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
