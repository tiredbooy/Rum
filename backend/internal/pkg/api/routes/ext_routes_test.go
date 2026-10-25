package routes

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/tiredbooy/Rum/backend/internal/pkg/api/handlers"
	"github.com/tiredbooy/Rum/backend/internal/pkg/api/middlewares"
	"github.com/tiredbooy/Rum/backend/internal/pkg/config"
	"github.com/tiredbooy/Rum/backend/internal/pkg/download"
)

func extApp(t *testing.T) (*gin.Engine, string) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	out := t.TempDir()
	handlers.InitAPI(&download.Options{Parallel: 1, Connections: 1, Out: out, Silent: true})
	t.Cleanup(func() { handlers.GlobalManager.Shutdown() })
	tok, err := config.ExtensionToken()
	if err != nil {
		t.Fatal(err)
	}
	r := gin.New()
	middlewares.SetupMiddlewares(r)
	SetupRouter(r)
	return r, tok
}

func call(r *gin.Engine, method, path, origin, token, body string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	req.Host = "127.0.0.1:8080"
	req.RemoteAddr = "127.0.0.1:5555"
	req.Header.Set("Content-Type", "application/json")
	if origin != "" {
		req.Header.Set("Origin", origin)
	}
	if token != "" {
		req.Header.Set(middlewares.ExtTokenHeader, token)
	}
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w
}

func TestExtRoutesRequireTokenAndExtensionOrigin(t *testing.T) {
	r, tok := extApp(t)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Length", "5")
		w.Header().Set("Accept-Ranges", "bytes")
	}))
	defer srv.Close()
	add := `{"url":"` + srv.URL + `/f.bin","auto_start":false,"cookie":"s=1"}`

	if c := call(r, "GET", "/ext/ping", "", "", "").Code; c != 401 {
		t.Errorf("no token: %d", c)
	}
	if c := call(r, "GET", "/ext/ping", "chrome-extension://abc", "bad", "").Code; c != 401 {
		t.Errorf("wrong token: %d", c)
	}
	if c := call(r, "GET", "/ext/ping", "https://evil.example", tok, "").Code; c != 403 {
		t.Errorf("web origin w/ token: %d", c)
	}
	if c := call(r, "POST", "/ext/add", "https://evil.example", tok, add).Code; c != 403 {
		t.Errorf("web origin add: %d", c)
	}
	if c := call(r, "GET", "/ext/ping", "moz-extension://uuid", tok, "").Code; c != 200 {
		t.Errorf("good ping: %d", c)
	}
	w := call(r, "POST", "/ext/add", "chrome-extension://abc", tok, add)
	if w.Code != 201 || len(handlers.GlobalManager.GetAllJobs()) != 1 {
		t.Fatalf("add: %d %s", w.Code, w.Body.String())
	}
	if got := handlers.GlobalManager.GetAllJobs()[0].GetCookie(); got != "s=1" {
		t.Errorf("cookie = %q", got)
	}
}

func TestExtensionOriginCannotReachMainAPI(t *testing.T) {
	r, tok := extApp(t)
	for _, p := range []string{"/api/v1/downloads", "/api/v1/settings", "/api/v1/extension/token"} {
		w := call(r, "GET", p, "chrome-extension://abc", tok, "")
		if w.Code != 403 {
			t.Errorf("%s from extension origin: %d, want 403", p, w.Code)
		}
	}
}

func TestExtPreflightFromExtension(t *testing.T) {
	r, _ := extApp(t)
	req := httptest.NewRequest("OPTIONS", "/ext/add", nil)
	req.Host = "127.0.0.1:8080"
	req.Header.Set("Origin", "chrome-extension://abc")
	req.Header.Set("Access-Control-Request-Method", "POST")
	req.Header.Set("Access-Control-Request-Headers", "x-rum-token,content-type")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != 204 {
		t.Fatalf("preflight %d", w.Code)
	}
}
