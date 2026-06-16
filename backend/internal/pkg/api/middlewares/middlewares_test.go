package middlewares

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/tiredbooy/Rum/backend/internal/pkg/api/dto"
)

func init() {
	gin.SetMode(gin.TestMode)
}

// TestRequestIDGenerated verifies the middleware sets a request ID in both the
// gin context and the response header when the client supplies none.
func TestRequestIDGenerated(t *testing.T) {
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "/", nil)

	var ctxID string
	h := RequestID()
	h(c)
	ctxID = c.GetString(RequestIDContextKey)

	hdrID := w.Header().Get(RequestIDHeader)
	if hdrID == "" {
		t.Fatal("response header X-Request-ID not set")
	}
	if ctxID == "" {
		t.Fatal("request_id not stored in gin context")
	}
	if ctxID != hdrID {
		t.Fatalf("context id %q != header id %q", ctxID, hdrID)
	}
}

// TestRequestIDHonorsInbound verifies an inbound X-Request-ID is propagated.
func TestRequestIDHonorsInbound(t *testing.T) {
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "/", nil)
	c.Request.Header.Set(RequestIDHeader, "trace-123")

	RequestID()(c)

	if got := c.GetString(RequestIDContextKey); got != "trace-123" {
		t.Fatalf("context id = %q, want trace-123", got)
	}
	if got := w.Header().Get(RequestIDHeader); got != "trace-123" {
		t.Fatalf("header id = %q, want trace-123", got)
	}
}

// TestRecoveryReturns500 verifies a panic downstream is converted into a
// sanitized 500 envelope and never leaks the panic value to the client.
func TestRecoveryReturns500(t *testing.T) {
	r := gin.New()
	r.Use(Recovery())
	r.GET("/boom", func(c *gin.Context) { panic("secret internal detail") })

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/boom", nil)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want 500", w.Code)
	}
	var resp dto.ErrorResponse
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("not an error envelope: %v (%s)", err, w.Body.String())
	}
	if resp.Code != dto.CodeInternal {
		t.Fatalf("code = %q, want %q", resp.Code, dto.CodeInternal)
	}
	if got := w.Body.String(); got == "" || contains(got, "secret internal detail") {
		t.Fatalf("panic detail leaked to client: %s", got)
	}
}

// TestRateLimitReturns429PastBurst verifies the limiter rejects with 429 + the
// standard envelope + a Retry-After header once the burst is exhausted.
func TestRateLimitReturns429PastBurst(t *testing.T) {
	const burst = 3
	r := gin.New()
	// rps=1 so refills don't sneak in within the tight test loop.
	r.Use(RateLimit(RateLimitConfig{RPS: 1, Burst: burst}))
	r.GET("/", func(c *gin.Context) { c.String(http.StatusOK, "ok") })

	var got429 bool
	for i := 0; i < burst+2; i++ {
		w := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		req.RemoteAddr = "10.1.1.1:1234"
		r.ServeHTTP(w, req)
		if i < burst {
			if w.Code != http.StatusOK {
				t.Fatalf("request %d within burst got %d, want 200", i, w.Code)
			}
			continue
		}
		// past burst
		if w.Code != http.StatusTooManyRequests {
			t.Fatalf("request %d past burst got %d, want 429", i, w.Code)
		}
		got429 = true
		if w.Header().Get("Retry-After") == "" {
			t.Fatal("429 response missing Retry-After header")
		}
		var resp dto.ErrorResponse
		if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
			t.Fatalf("429 not an error envelope: %v (%s)", err, w.Body.String())
		}
		if resp.Code != dto.CodeRateLimited {
			t.Fatalf("code = %q, want %q", resp.Code, dto.CodeRateLimited)
		}
	}
	if !got429 {
		t.Fatal("expected at least one 429 past burst")
	}
}

// TestRateLimitPerIPIsolation verifies one noisy IP does not throttle another.
func TestRateLimitPerIPIsolation(t *testing.T) {
	r := gin.New()
	r.Use(RateLimit(RateLimitConfig{RPS: 1, Burst: 1}))
	r.GET("/", func(c *gin.Context) { c.String(http.StatusOK, "ok") })

	exhaust := func(ip string) int {
		w := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		req.RemoteAddr = ip + ":1000"
		r.ServeHTTP(w, req)
		return w.Code
	}

	// First IP burns its single token, then is throttled.
	if code := exhaust("10.0.0.1"); code != http.StatusOK {
		t.Fatalf("first request for IP1 = %d, want 200", code)
	}
	if code := exhaust("10.0.0.1"); code != http.StatusTooManyRequests {
		t.Fatalf("second request for IP1 = %d, want 429", code)
	}
	// A different IP still has its own full bucket.
	if code := exhaust("10.0.0.2"); code != http.StatusOK {
		t.Fatalf("first request for IP2 = %d, want 200 (per-IP isolation broken)", code)
	}
}

func contains(s, sub string) bool {
	return len(sub) > 0 && len(s) >= len(sub) && indexOf(s, sub) >= 0
}

func indexOf(s, sub string) int {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return i
		}
	}
	return -1
}
