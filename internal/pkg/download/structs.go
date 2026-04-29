package download

import (
	"context"
)

type DownloadTask struct {
	ID       string
	URL      string
	Attempts int
	Options
	Downloaded int64
	TotalSize  int64
	Cancel     context.CancelFunc
	Status     string
}

type Options struct {
	SpeedLimit int
	Out        string
	Parallel   int

	WantGroupFolder bool
	GroupFolder     string

	Referer   string
	UserAgent string

	MaxRetries int
	Silent     bool
}

type DownloadResult struct {
	URL     string
	Success bool
	Error   error
}

type DownloadTargets struct {
	FileName string
	FileSize int64

	DownloadSpeed   float64
	DownloadedBytes int64
}
