package handlers

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/tiredbooy/Rum/backend/internal/pkg/api/dto"
)

// TestStreamLoopCoalesces verifies the SSE pump collapses a flood of per-read
// progress updates into a small number of frames carrying the LATEST value per
// job — the fix for the stuttering/"not live" progress bar. Without coalescing a
// 500-update flood would emit ~500 frames; with it, the client sees only the
// latest (downloaded=500) in a handful of flushes.
func TestStreamLoopCoalesces(t *testing.T) {
	gin.SetMode(gin.TestMode)
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	c.Request = httptest.NewRequest(http.MethodGet, "/stream", nil).WithContext(ctx)

	flusher, ok := c.Writer.(http.Flusher)
	if !ok {
		t.Fatal("gin response writer is not an http.Flusher")
	}

	ch := make(chan dto.ProgressUpdate, 1024)
	done := make(chan struct{})
	go func() { streamLoop(c, flusher, ch); close(done) }()

	// Flood 500 updates for one job, then close so streamLoop flushes the
	// coalesced latest and returns.
	for i := 1; i <= 500; i++ {
		ch <- dto.ProgressUpdate{JobID: "x", Downloaded: int64(i), TotalSize: 500, Progress: i / 5}
	}
	close(ch)
	<-done

	body := rec.Body.String()
	frames := strings.Count(body, "data: ")
	if frames == 0 {
		t.Fatal("expected at least one data frame")
	}
	if frames > 50 {
		t.Errorf("expected coalesced output (<=50 frames) for a 500-update flood, got %d", frames)
	}
	if !strings.Contains(body, `"downloaded":500`) {
		t.Errorf("latest update (downloaded=500) was not flushed; body=%q", body[:min(len(body), 300)])
	}
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
