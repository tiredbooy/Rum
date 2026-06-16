package handlers

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/tiredbooy/Rum/backend/internal/pkg/api/dto"
)

func init() {
	gin.SetMode(gin.TestMode)
}

// TestCreateDownloadValidationEnvelope verifies that a malformed/invalid body is
// rejected with the consistent error envelope BEFORE the engine is touched.
func TestCreateDownloadValidationEnvelope(t *testing.T) {
	tests := []struct {
		name     string
		body     string
		wantCode int
		wantErr  string // dto error code
	}{
		{
			name:     "invalid json",
			body:     `{not json`,
			wantCode: http.StatusBadRequest,
			wantErr:  dto.CodeValidation,
		},
		{
			name:     "missing urls",
			body:     `{"urls": []}`,
			wantCode: http.StatusBadRequest,
			wantErr:  dto.CodeValidation,
		},
		{
			name:     "bad url scheme",
			body:     `{"urls": ["ftp://example.com/x"]}`,
			wantCode: http.StatusBadRequest,
			wantErr:  dto.CodeValidation,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(w)
			c.Request = httptest.NewRequest(http.MethodPost, "/api/v1/downloads", strings.NewReader(tt.body))
			c.Request.Header.Set("Content-Type", "application/json")

			CreateDownload(c)

			if w.Code != tt.wantCode {
				t.Fatalf("status = %d, want %d (body: %s)", w.Code, tt.wantCode, w.Body.String())
			}
			var resp dto.ErrorResponse
			if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
				t.Fatalf("response is not an error envelope: %v (%s)", err, w.Body.String())
			}
			if resp.Error == "" {
				t.Fatal("error envelope missing 'error' field")
			}
			if tt.wantErr != "" && resp.Code != tt.wantErr {
				t.Fatalf("code = %q, want %q", resp.Code, tt.wantErr)
			}
		})
	}
}

// TestUpdateSpeedLimitValidation verifies the focused speed-limit endpoint
// returns field-level validation errors for bad input.
func TestUpdateSpeedLimitValidation(t *testing.T) {
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPut, "/api/v1/settings/speed-limit", strings.NewReader(`{"speed_limit_kb": -3}`))
	c.Request.Header.Set("Content-Type", "application/json")

	UpdateSpeedLimit(c)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400 (body: %s)", w.Code, w.Body.String())
	}
	var resp dto.ErrorResponse
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("not an error envelope: %v", err)
	}
	if resp.Code != dto.CodeValidation {
		t.Fatalf("code = %q, want %q", resp.Code, dto.CodeValidation)
	}
	if _, ok := resp.Fields["speed_limit_kb"]; !ok {
		t.Fatalf("expected field-level error for speed_limit_kb, got %v", resp.Fields)
	}
}

func TestWriteServiceErrorMapping(t *testing.T) {
	cases := []struct {
		msg      string
		wantCode int
	}{
		{"job abc not found", http.StatusNotFound},
		{"job abc is already running", http.StatusConflict},
		{"job abc is not running", http.StatusConflict},
		{"no URLs provided", http.StatusBadRequest},
		{"something exploded internally", http.StatusInternalServerError},
	}
	for _, tt := range cases {
		t.Run(tt.msg, func(t *testing.T) {
			w := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(w)
			c.Request = httptest.NewRequest(http.MethodGet, "/", nil)
			writeServiceError(c, errString(tt.msg))
			if w.Code != tt.wantCode {
				t.Fatalf("msg %q -> status %d, want %d", tt.msg, w.Code, tt.wantCode)
			}
			// Internal errors must not leak the raw message.
			if tt.wantCode == http.StatusInternalServerError &&
				strings.Contains(w.Body.String(), "exploded") {
				t.Fatalf("internal error leaked raw message: %s", w.Body.String())
			}
		})
	}
}

type errString string

func (e errString) Error() string { return string(e) }

// TestSetDownloadPriorityValidation verifies the priority endpoint rejects a bad
// body with the standard validation envelope BEFORE reaching the manager (so it
// does not depend on the orchestrator-added SetJobPriority method).
func TestSetDownloadPriorityValidation(t *testing.T) {
	cases := []struct {
		name string
		body string
	}{
		{"invalid json", `{not json`},
		{"missing priority", `{}`},
		{"unknown priority", `{"priority":"urgent"}`},
	}
	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			w := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(w)
			c.Params = gin.Params{{Key: "id", Value: "abc"}}
			c.Request = httptest.NewRequest(http.MethodPatch, "/api/v1/downloads/abc/priority", strings.NewReader(tt.body))
			c.Request.Header.Set("Content-Type", "application/json")

			SetDownloadPriority(c)

			if w.Code != http.StatusBadRequest {
				t.Fatalf("status = %d, want 400 (body: %s)", w.Code, w.Body.String())
			}
			var resp dto.ErrorResponse
			if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
				t.Fatalf("not an error envelope: %v (%s)", err, w.Body.String())
			}
			if resp.Code != dto.CodeValidation {
				t.Fatalf("code = %q, want %q", resp.Code, dto.CodeValidation)
			}
		})
	}
}
