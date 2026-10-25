package dto

type ProgressUpdate struct {
	JobID      string  `json:"job_id"`
	Downloaded int64   `json:"downloaded"`
	TotalSize  int64   `json:"total_size"`
	Speed      float64 `json:"speed"`
	Status     string  `json:"status"`
	Progress   int     `json:"progress"`
	ETA        int64   `json:"eta"`
	// Set on the frame that ends a job with an error, so the card can show why
	// without a refetch. The disk fields go with ErrorCode "disk_full".
	Error       string `json:"error,omitempty"`
	ErrorCode   string `json:"error_code,omitempty"`
	NeededBytes int64  `json:"needed_bytes,omitempty"`
	FreeBytes   int64  `json:"free_bytes,omitempty"`
	DiskPath    string `json:"disk_path,omitempty"`
}
