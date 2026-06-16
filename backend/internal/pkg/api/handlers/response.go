package handlers

import (
	"log"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/tiredbooy/Rum/backend/internal/pkg/api/dto"
)

// writeError emits the single, consistent error envelope.
// `msg` must be a sanitized, client-safe message — never a raw internal error.
func writeError(c *gin.Context, status int, code, msg string) {
	c.JSON(status, dto.ErrorResponse{Error: msg, Code: code})
}

// writeFieldErrors emits a 400 validation envelope with per-field detail.
func writeFieldErrors(c *gin.Context, msg string, fields map[string]string) {
	c.JSON(http.StatusBadRequest, dto.ErrorResponse{
		Error:  msg,
		Code:   dto.CodeValidation,
		Fields: fields,
	})
}

// writeServiceError maps an engine/service error to an honest HTTP status and a
// sanitized envelope. The full error is logged server-side with the request ID
// so we get observability without leaking internals to the client.
//
// The engine returns plain fmt.Errorf strings (no sentinel errors), so we
// classify by inspecting the message. This stays best-effort and defaults to
// 500 for anything unrecognized.
func writeServiceError(c *gin.Context, err error) {
	if err == nil {
		return
	}
	reqID := c.GetString("request_id")
	log.Printf("[request_id=%s] service error: %v", reqID, err)

	msg := strings.ToLower(err.Error())

	switch {
	case strings.Contains(msg, "not found"):
		writeError(c, http.StatusNotFound, dto.CodeNotFound, "resource not found")
	case strings.Contains(msg, "already") ||
		strings.Contains(msg, "is not running") ||
		strings.Contains(msg, "exists"):
		// "already running/completed", "is not running", duplicate, etc.
		writeError(c, http.StatusConflict, dto.CodeConflict, sanitizeConflict(err))
	case strings.Contains(msg, "no urls") ||
		strings.Contains(msg, "no valid jobs") ||
		strings.Contains(msg, "not valid") ||
		strings.Contains(msg, "invalid"):
		writeError(c, http.StatusBadRequest, dto.CodeValidation, err.Error())
	default:
		// Unknown: do not leak the raw error to the client.
		writeError(c, http.StatusInternalServerError, dto.CodeInternal, "internal server error")
	}
}

// sanitizeConflict keeps state-transition messages (they are user-meaningful and
// safe), but never echoes unexpected internals.
func sanitizeConflict(err error) string {
	m := err.Error()
	if len(m) > 200 {
		m = m[:200]
	}
	return m
}
