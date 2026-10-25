package handlers

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
)

// StatusSource reports live internet reachability (the reconnect controller).
type StatusSource interface {
	Status() (online bool, lastProbeAt time.Time)
}

// GlobalResumer is set at startup; nil means "no measurement" and reads online.
var GlobalResumer StatusSource

// statusResponse is the GET /status body.
type statusResponse struct {
	Online      bool       `json:"online"`
	LastProbeAt *time.Time `json:"last_probe_at,omitempty"`
}

// GetStatus returns current connectivity as measured through the engine's proxy.
func GetStatus(c *gin.Context) {
	resp := statusResponse{Online: true}
	if GlobalResumer != nil {
		online, at := GlobalResumer.Status()
		resp.Online = online
		if !at.IsZero() {
			resp.LastProbeAt = &at
		}
	}
	c.JSON(http.StatusOK, resp)
}
