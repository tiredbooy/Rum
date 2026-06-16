package handlers

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/tiredbooy/Rum/backend/internal/pkg/api/dto"
)

// sseKeepAlive is how often we send a comment line to keep the connection alive
// and to detect dead clients (a failed write surfaces via context cancellation).
const sseKeepAlive = 15 * time.Second

func setSSEHeaders(c *gin.Context) {
	c.Header("Content-Type", "text/event-stream")
	c.Header("Cache-Control", "no-cache")
	c.Header("Connection", "keep-alive")
	// Disable proxy buffering (nginx) so events are delivered immediately.
	c.Header("X-Accel-Buffering", "no")
}

// streamLoop is the shared SSE pump. It writes updates from updateCh until the
// client disconnects (ctx done) or the channel is closed, sending periodic
// keep-alive comments in between. The caller owns subscription/unsubscription.
func streamLoop(c *gin.Context, flusher http.Flusher, updateCh <-chan dto.ProgressUpdate) {
	ctx := c.Request.Context()

	// Establish the stream immediately so clients see a 200 + open connection.
	fmt.Fprint(c.Writer, ": connected\n\n")
	flusher.Flush()

	ticker := time.NewTicker(sseKeepAlive)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case update, ok := <-updateCh:
			if !ok {
				return
			}
			data, err := json.Marshal(update)
			if err != nil {
				continue
			}
			if _, err := fmt.Fprintf(c.Writer, "data: %s\n\n", data); err != nil {
				return
			}
			flusher.Flush()
		case <-ticker.C:
			if _, err := fmt.Fprint(c.Writer, ": keep-alive\n\n"); err != nil {
				return
			}
			flusher.Flush()
		}
	}
}

func StreamProgress(c *gin.Context) {
	jobID := c.Param("id")

	flusher, ok := c.Writer.(http.Flusher)
	if !ok {
		writeError(c, http.StatusInternalServerError, dto.CodeUnsupported, "streaming unsupported")
		return
	}

	setSSEHeaders(c)

	// Subscribe AFTER we know we can stream, and always unsubscribe on return so
	// the per-job subscriber slice cannot leak on client disconnect.
	updateCh := GlobalManager.Subscribe(jobID)
	defer GlobalManager.UnSubscribe(jobID, updateCh)

	streamLoop(c, flusher, updateCh)
}

func StreamAllProgress(c *gin.Context) {
	flusher, ok := c.Writer.(http.Flusher)
	if !ok {
		writeError(c, http.StatusInternalServerError, dto.CodeUnsupported, "streaming unsupported")
		return
	}

	setSSEHeaders(c)

	updateCh := GlobalManager.SubscribeAll()
	defer GlobalManager.UnSubscribeAll(updateCh)

	streamLoop(c, flusher, updateCh)
}
