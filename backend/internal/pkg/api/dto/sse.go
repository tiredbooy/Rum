package dto

type ProgressUpdate struct {
	JobID      string  `json:"job_id"`
	Downloaded int64   `json:"downloaded"`
	TotalSize  int64   `json:"total_size"`
	Speed      float64 `json:"speed"`
	Status     string  `json:"status"`
	Progress   int     `json:"progress"`
	ETA        int64   `json:"eta"`
}
