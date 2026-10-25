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

// stateSource is the optional richer view: the three-way classification and a
// "window is visible" signal that enables idle checks.
type stateSource interface {
	ConnState() string
	Touch()
}

// GlobalResumer is set at startup; nil means "no measurement" and reads online.
var GlobalResumer StatusSource

// statusResponse is the GET /status body.
type statusResponse struct {
	Online bool `json:"online"`
	// State is "online", "site_blocked" (internet works, a download host does
	// not) or "offline". Online is false only for "offline".
	State       string     `json:"state"`
	LastProbeAt *time.Time `json:"last_probe_at,omitempty"`
}

// GetStatus returns current connectivity as measured through the engine's proxy.
func GetStatus(c *gin.Context) {
	resp := statusResponse{Online: true, State: "online"}
	if GlobalResumer != nil {
		online, at := GlobalResumer.Status()
		resp.Online = online
		if ss, ok := GlobalResumer.(stateSource); ok {
			ss.Touch()
			resp.State = ss.ConnState()
		} else if !online {
			resp.State = "offline"
		}
		if !at.IsZero() {
			resp.LastProbeAt = &at
		}
	}
	c.JSON(http.StatusOK, resp)
}
