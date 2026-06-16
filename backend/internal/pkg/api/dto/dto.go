package dto

import (
	"fmt"
	"net/url"
	"strings"
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

	if len(fields) == 0 {
		return nil
	}
	return fields
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
