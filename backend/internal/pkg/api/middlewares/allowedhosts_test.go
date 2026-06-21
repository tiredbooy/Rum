package middlewares

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestIsLoopbackHost(t *testing.T) {
	allow := []string{
		"127.0.0.1:8080", "127.0.0.1", "127.0.0.5:9000",
		"localhost:8080", "localhost", "[::1]:8080", "::1", "",
	}
	for _, h := range allow {
		if !isLoopbackHost(h) {
			t.Errorf("host %q should be allowed (loopback)", h)
		}
	}
	deny := []string{
		"evil.com", "evil.com:8080", "attacker.example:8080",
		"10.0.0.1:8080", "192.168.1.5", "169.254.1.1:8080", "8.8.8.8:80",
	}
	for _, h := range deny {
		if isLoopbackHost(h) {
			t.Errorf("host %q should be rejected (non-loopback)", h)
		}
	}
}

// TestAllowedHostsMiddleware guards the anti-DNS-rebind behavior end to end.
func TestAllowedHostsMiddleware(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(AllowedHosts())
	r.GET("/x", func(c *gin.Context) { c.String(http.StatusOK, "ok") })

	// Legit loopback request from the desktop frontend.
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest("GET", "http://127.0.0.1:8080/x", nil))
	if w.Code != http.StatusOK {
		t.Fatalf("loopback host: got %d, want 200", w.Code)
	}

	// DNS-rebinding: attacker's domain re-resolved to 127.0.0.1, so the dial
	// succeeds but the Host header is the attacker's domain.
	req := httptest.NewRequest("GET", "http://127.0.0.1:8080/x", nil)
	req.Host = "attacker.example"
	w = httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusForbidden {
		t.Fatalf("rebind host: got %d, want 403", w.Code)
	}
}
