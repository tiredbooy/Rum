package dto

import (
	"fmt"
	"net/url"
	"strings"
	"time"

	"github.com/tiredbooy/Rum/backend/internal/pkg/config"
)

// Data Transfer Object (DTO)

type CreateDownloadRequest struct {
	URLs        []string `json:"urls" binding:"required,min=1"`
	DestPath    string   `json:"dest_path"`
	Filename    *string  `json:"filename"`
	SpeedLimit  int      `json:"speed_limit"`
	UserAgent   string   `json:"user_agent"`
	Referer     string   `json:"referer"`
	GroupFolder string   `json:"group_folder"`
	MaxRetries  int      `json:"max_retries"`
	AutoStart   bool     `json:"auto_start"`

	// Optional, additive fields (existing clients omit them safely).
	Checksum     string `json:"checksum,omitempty"`
	ChecksumAlgo string `json:"checksum_algo,omitempty"` // "sha256" | "md5"
	StartAt      string `json:"start_at,omitempty"`      // RFC3339
	Category     string `json:"category,omitempty"`
}

// Validate performs field-level validation at the edge, before anything reaches
// the engine. It returns a map of field -> problem so the handler can return all
// problems at once (go-api-design: return field-level errors when practical).
func (r *CreateDownloadRequest) Validate() map[string]string {
	fields := map[string]string{}

	if len(r.URLs) == 0 {
		fields["urls"] = "at least one URL is required"
	}
	for i, raw := range r.URLs {
		raw = strings.TrimSpace(raw)
		if raw == "" {
			fields[fmt.Sprintf("urls[%d]", i)] = "URL must not be empty"
			continue
		}
		u, err := url.Parse(raw)
		if err != nil || u.Scheme == "" || u.Host == "" {
			fields[fmt.Sprintf("urls[%d]", i)] = "must be an absolute http(s) URL"
			continue
		}
		if u.Scheme != "http" && u.Scheme != "https" {
			fields[fmt.Sprintf("urls[%d]", i)] = "scheme must be http or https"
		}
	}

	if r.SpeedLimit < 0 {
		fields["speed_limit"] = "must be >= 0"
	}
	if r.MaxRetries < 0 {
		fields["max_retries"] = "must be >= 0"
	}

	// Optional checksum: if a checksum is provided, the algo must be supported.
	if strings.TrimSpace(r.Checksum) != "" {
		switch strings.ToLower(strings.TrimSpace(r.ChecksumAlgo)) {
		case "sha256", "md5":
		case "":
			fields["checksum_algo"] = `is required when checksum is set ("sha256" or "md5")`
		default:
			fields["checksum_algo"] = `must be "sha256" or "md5"`
		}
	}

	// Optional start_at: must be RFC3339 when present.
	if strings.TrimSpace(r.StartAt) != "" {
		if _, err := time.Parse(time.RFC3339, strings.TrimSpace(r.StartAt)); err != nil {
			fields["start_at"] = "must be an RFC3339 timestamp"
		}
	}

	if len(fields) == 0 {
		return nil
	}
	return fields
}

// ParsedStartAt returns the parsed StartAt (zero time when empty/invalid). Call
// Validate first to surface a bad value to the client.
func (r *CreateDownloadRequest) ParsedStartAt() time.Time {
	s := strings.TrimSpace(r.StartAt)
	if s == "" {
		return time.Time{}
	}
	t, err := time.Parse(time.RFC3339, s)
	if err != nil {
		return time.Time{}
	}
	return t
}

type DownloadResponse struct {
	ID          string  `json:"id"`
	URL         string  `json:"url"`
	Filename    string  `json:"filename"`
	Status      string  `json:"status"`
	Progress    int     `json:"progress"`
	Downloaded  int64   `json:"downloaded"`
	TotalSize   int64   `json:"total_size"`
	Speed       float64 `json:"speed"`
	Remaining   int64   `json:"remaining"`
	Error       string  `json:"error,omitempty"`
	CreatedAt   string  `json:"created_at"`
	CompletedAt string  `json:"completed_at,omitempty"`
}

type BatchCreateResponse struct {
	Jobs []JobInfo `json:"jobs"`
}

type JobInfo struct {
	ID     string `json:"id"`
	URL    string `json:"url"`
	Status string `json:"status"`
}

// Priority levels accepted by PATCH /downloads/:id/priority. These string
// values are part of the API contract and are passed straight through to the
// engine's SetJobPriority.
const (
	PriorityLow    = "low"
	PriorityNormal = "normal"
	PriorityHigh   = "high"
)

// SetPriorityRequest changes the scheduling priority of a single download.
type SetPriorityRequest struct {
	Priority string `json:"priority"`
}

// Validate ensures the priority is one of the accepted levels.
func (r *SetPriorityRequest) Validate() map[string]string {
	switch r.Priority {
	case PriorityLow, PriorityNormal, PriorityHigh:
		return nil
	case "":
		return map[string]string{"priority": "is required"}
	default:
		return map[string]string{"priority": `must be one of "low", "normal", "high"`}
	}
}

// SpeedLimitRequest updates the global download speed limit (KB/s, 0 = unlimited).
type SpeedLimitRequest struct {
	SpeedLimitKB *int `json:"speed_limit_kb"`
}

func (r *SpeedLimitRequest) Validate() map[string]string {
	if r.SpeedLimitKB == nil {
		return map[string]string{"speed_limit_kb": "is required"}
	}
	if *r.SpeedLimitKB < 0 {
		return map[string]string{"speed_limit_kb": "must be >= 0 (0 means unlimited)"}
	}
	return nil
}

// ScheduleSettings is the body/response for GET|PUT /settings/schedule. It reuses
// config.SpeedRule (start_hour/end_hour/limit_kbps/days) so there is one rule
// shape across the codebase.
type ScheduleSettings struct {
	ScheduledStartEnabled bool               `json:"scheduled_start_enabled"`
	Rules                 []config.SpeedRule `json:"rules"`
}

// Validate performs light edge validation; config.Setting.Validate does the
// authoritative clamping (hours, days, limits) on Save.
func (s *ScheduleSettings) Validate() map[string]string {
	fields := map[string]string{}
	for i, r := range s.Rules {
		if r.StartHour < 0 || r.StartHour > 23 {
			fields[fmt.Sprintf("rules[%d].start_hour", i)] = "must be 0..23"
		}
		if r.EndHour < 0 || r.EndHour > 23 {
			fields[fmt.Sprintf("rules[%d].end_hour", i)] = "must be 0..23"
		}
		if r.LimitKBps < 0 {
			fields[fmt.Sprintf("rules[%d].limit_kbps", i)] = "must be >= 0 (0 means unlimited)"
		}
		for j, d := range r.Days {
			if d < 0 || d > 6 {
				fields[fmt.Sprintf("rules[%d].days[%d]", i, j)] = "must be 0..6 (0=Sun..6=Sat)"
			}
		}
	}
	if len(fields) == 0 {
		return nil
	}
	return fields
}

// CategorySettings is the body/response for GET|PUT /settings/categories. It
// reuses config.CategoryRule so there is one rule shape across the codebase.
type CategorySettings struct {
	Enabled bool                  `json:"enabled"`
	Rules   []config.CategoryRule `json:"rules"`
}

// Validate ensures each rule has a name and at least one extension before it is
// persisted (config.Setting.Validate also prunes invalid rules defensively).
func (s *CategorySettings) Validate() map[string]string {
	fields := map[string]string{}
	for i, r := range s.Rules {
		if strings.TrimSpace(r.Name) == "" {
			fields[fmt.Sprintf("rules[%d].name", i)] = "is required"
		}
		if len(r.Extensions) == 0 {
			fields[fmt.Sprintf("rules[%d].extensions", i)] = "at least one extension is required"
		}
	}
	if len(fields) == 0 {
		return nil
	}
	return fields
}

// BatchCreateRequest is the body for POST /downloads/batch: a list of URLs plus
// optional shared options applied to every created job.
type BatchCreateRequest struct {
	URLs    []string            `json:"urls" binding:"required,min=1"`
	Options *BatchCreateOptions `json:"options,omitempty"`
}

// BatchCreateOptions are the per-batch options (mirrors the optional fields of
// CreateDownloadRequest). All fields are optional.
type BatchCreateOptions struct {
	DestPath     string `json:"dest_path"`
	SpeedLimit   int    `json:"speed_limit"`
	MaxRetries   int    `json:"max_retries"`
	GroupFolder  string `json:"group_folder"`
	Category     string `json:"category"`
	Checksum     string `json:"checksum"`
	ChecksumAlgo string `json:"checksum_algo"`
	StartAt      string `json:"start_at"` // RFC3339
	AutoStart    bool   `json:"auto_start"`
}

// BatchCreateResult is one per-URL outcome in the batch response.
type BatchURLError struct {
	URL   string `json:"url"`
	Error string `json:"error"`
}

// BatchCreateResultResponse is the response for POST /downloads/batch.
type BatchCreateResultResponse struct {
	Created []DownloadResponse `json:"created"`
	Errors  []BatchURLError    `json:"errors"`
}
