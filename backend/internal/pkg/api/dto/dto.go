package dto

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
