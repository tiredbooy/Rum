package middlewares

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
)

const testExtToken = "rum_secret"

func extRouter() *gin.Engine {
	r := gin.New()
	r.Use(ExtensionAwareCors(Cors(nil)))
	g := r.Group("/ext", ExtGuard(func(s string) bool { return s == testExtToken }))
	g.GET("/ping", func(c *gin.Context) { c.String(200, "pong") })
	r.GET("/api/v1/other", func(c *gin.Context) { c.String(200, "other") })
	return r
}

func do(r *gin.Engine, method, path, origin, token string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(method, path, nil)
	if origin != "" {
		req.Header.Set("Origin", origin)
	}
	if token != "" {
		req.Header.Set(ExtTokenHeader, token)
	}
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w
}

func TestExtGuardToken(t *testing.T) {
	r := extRouter()
	cases := []struct {
		name, token string
		want        int
	}{
		{"missing", "", 401},
		{"wrong", "nope", 401},
		{"prefix of right", testExtToken[:5], 401},
		{"right", testExtToken, 200},
	}
	for _, tc := range cases {
		if got := do(r, "GET", "/ext/ping", "", tc.token).Code; got != tc.want {
			t.Errorf("%s: status %d, want %d", tc.name, got, tc.want)
		}
	}
}

func TestExtGuardOriginRules(t *testing.T) {
	r := extRouter()
	for _, o := range []string{"chrome-extension://abcdef", "moz-extension://1234-uuid"} {
		w := do(r, "GET", "/ext/ping", o, testExtToken)
		if w.Code != 200 {
			t.Errorf("%s with token: %d, want 200", o, w.Code)
		}
		if w.Header().Get("Access-Control-Allow-Origin") != o {
			t.Errorf("%s not echoed in ACAO: %q", o, w.Header().Get("Access-Control-Allow-Origin"))
		}
		if do(r, "GET", "/ext/ping", o, "").Code != 401 {
			t.Errorf("%s without token must be 401", o)
		}
	}
	// A web page is refused even with a valid token.
	for _, o := range []string{"https://evil.example", "http://127.0.0.1:8080", "null", "http://localhost:5173"} {
		if got := do(r, "GET", "/ext/ping", o, testExtToken).Code; got != 403 {
			t.Errorf("web origin %q: %d, want 403", o, got)
		}
	}
}

func TestExtensionOriginsOnlyAllowedOnExtPaths(t *testing.T) {
	r := extRouter()
	w := do(r, "GET", "/api/v1/other", "chrome-extension://abcdef", "")
	if w.Code != 403 {
		t.Fatalf("extension origin on non-/ext path: %d, want 403", w.Code)
	}
	if w.Header().Get("Access-Control-Allow-Origin") != "" {
		t.Fatal("ACAO leaked on non-/ext path")
	}
	// The existing allowlist still works elsewhere.
	if do(r, "GET", "/api/v1/other", "wails://wails", "").Code != 200 {
		t.Fatal("wails origin broken by extension CORS wrapper")
	}
	if do(r, "GET", "/api/v1/other", "https://evil.example", "").Code != 403 {
		t.Fatal("foreign origin must stay blocked")
	}
}

func TestExtPreflightAllowsTokenHeader(t *testing.T) {
	r := extRouter()
	req := httptest.NewRequest("OPTIONS", "/ext/ping", nil)
	req.Header.Set("Origin", "moz-extension://abc")
	req.Header.Set("Access-Control-Request-Method", "POST")
	req.Header.Set("Access-Control-Request-Headers", "x-rum-token,content-type")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != 204 {
		t.Fatalf("preflight status %d", w.Code)
	}
	if !strings.Contains(strings.ToLower(w.Header().Get("Access-Control-Allow-Headers")), "x-rum-token") {
		t.Fatalf("X-Rum-Token not allowed: %q", w.Header().Get("Access-Control-Allow-Headers"))
	}
}

func TestExtRateLimit(t *testing.T) {
	r := gin.New()
	g := r.Group("/ext", ExtRateLimit(RateLimitConfig{RPS: 1, Burst: 2}))
	g.GET("/ping", func(c *gin.Context) { c.String(200, "pong") })
	codes := []int{}
	for i := 0; i < 4; i++ {
		req := httptest.NewRequest(http.MethodGet, "/ext/ping", nil)
		req.RemoteAddr = "127.0.0.1:1234"
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		codes = append(codes, w.Code)
	}
	if codes[0] != 200 || codes[1] != 200 || codes[3] != 429 {
		t.Fatalf("codes = %v, want 200,200,..,429", codes)
	}
}

// The extension identifies Rum by this header BEFORE it ever sends the token, so
// the token is never handed to whatever else happens to listen on a nearby port.
func TestExtGuardIdentifiesRumWithoutToken(t *testing.T) {
	r := extRouter()
	for _, token := range []string{"", "wrong", testExtToken} {
		w := do(r, "GET", "/ext/ping", "", token)
		if w.Header().Get(ExtAppHeader) != "rum" {
			t.Errorf("token %q: %s header missing", token, ExtAppHeader)
		}
	}
	if do(r, "GET", "/api/v1/other", "", "").Header().Get(ExtAppHeader) != "" {
		t.Error("identity header must only be set on /ext/*")
	}
}
