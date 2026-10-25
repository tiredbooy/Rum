package dto

import (
	"strings"
	"time"

	"github.com/tiredbooy/Rum/backend/internal/pkg/config"
)

// maxNightQueueIDs bounds one bulk "move to night queue" request.
const maxNightQueueIDs = 5000

// NightPlanResponse is the body of GET|PUT /settings/night-plan: the saved plan
// plus its live state. The state fields are ignored on PUT.
type NightPlanResponse struct {
	config.NightPlan
	Active     bool      `json:"active"`
	CapReached bool      `json:"cap_reached"`
	NextStart  time.Time `json:"next_start,omitempty"`
}

// NightQueueRequest is the body of POST /downloads/night-queue: move downloads
// into (enabled) or out of the night queue.
type NightQueueRequest struct {
	IDs     []string `json:"ids"`
	Enabled bool     `json:"enabled"`
}

// Validate returns field -> problem, or nil.
func (r *NightQueueRequest) Validate() map[string]string {
	if len(r.IDs) == 0 {
		return map[string]string{"ids": "at least one id is required"}
	}
	if len(r.IDs) > maxNightQueueIDs {
		return map[string]string{"ids": "too many ids"}
	}
	for _, id := range r.IDs {
		if strings.TrimSpace(id) == "" {
			return map[string]string{"ids": "must not contain blank ids"}
		}
	}
	return nil
}
