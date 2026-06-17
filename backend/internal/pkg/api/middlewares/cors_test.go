package middlewares

import (
	"net/http"
	"net/http/httptest"
	"runtime"
	"testing"

	"github.com/gin-gonic/gin"
)

// These cases all previously panicked at startup ("bad origin: origins must
// contain '*' or include http://,https://") because the default origin list and
// the server's Cors([]string{"*"}) call reached gin-contrib/cors with a
// custom-scheme origin (tauri://localhost). Cors must build without panicking.
func TestCorsDoesNotPanic(t *testing.T) {
	cases := map[string][]string{
		"server default call": {"*"}, // what cmd/server/main.go passes
		"nil -> dev defaults": nil,   // includes tauri://localhost
		"empty slice":         {},    // -> dev defaults
		"explicit http":       {"http://localhost:5173"},
		"custom scheme only":  {"tauri://localhost"},
	}
	for name, origins := range cases {
		t.Run(name, func(t *testing.T) {
			defer func() {
				if r := recover(); r != nil {
					t.Fatalf("Cors(%v) panicked: %v", origins, r)
				}
			}()
			if h := Cors(origins); h == nil {
				t.Fatalf("Cors(%v) returned nil handler", origins)
			}
		})
	}
}

// TestCorsAllowDenyBehavior locks in the actual allow/deny decisions (the older
// test only checked that Cors() doesn't panic). It drives a real preflight through
// the middleware so a regression that allowed an evil origin, denied the Wails
// webview, or re-broadened the http://wails.localhost gate would fail here.
func TestCorsAllowDenyBehavior(t *testing.T) {
	gin.SetMode(gin.TestMode)

	preflight := func(origin string) (int, string) {
		r := gin.New()
		r.Use(Cors(nil))
		r.GET("/x", func(c *gin.Context) { c.String(http.StatusOK, "ok") })

		req := httptest.NewRequest(http.MethodOptions, "/x", nil)
		req.Header.Set("Origin", origin)
		req.Header.Set("Access-Control-Request-Method", "GET")
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		return w.Code, w.Header().Get("Access-Control-Allow-Origin")
	}

	// The app's own webview (custom scheme) and a dev origin are always allowed.
	for _, o := range []string{"wails://wails", "wails://wails.localhost", "http://localhost:5173"} {
		if code, acao := preflight(o); code != http.StatusNoContent || acao != o {
			t.Errorf("origin %q: want 204 + ACAO=%q, got %d + %q", o, o, code, acao)
		}
	}

	// Foreign / look-alike / null origins must be rejected (403, no ACAO).
	for _, o := range []string{"https://evil.example", "http://wails.localhost.evil.com", "http://wails.attacker", "null"} {
		if code, acao := preflight(o); code == http.StatusNoContent || acao != "" {
			t.Errorf("origin %q: want denied (not 204, empty ACAO), got %d + %q", o, code, acao)
		}
	}

	// http://wails.localhost is a plain-http origin a browser can forge; it is
	// allowed ONLY on Windows (WebView2), denied on Linux/macOS — see desktopOrigins.
	code, acao := preflight("http://wails.localhost")
	if runtime.GOOS == "windows" {
		if code != http.StatusNoContent || acao != "http://wails.localhost" {
			t.Errorf("windows: http://wails.localhost should be allowed, got %d + %q", code, acao)
		}
	} else if code == http.StatusNoContent || acao != "" {
		t.Errorf("%s: http://wails.localhost should be denied, got %d + %q", runtime.GOOS, code, acao)
	}
}
