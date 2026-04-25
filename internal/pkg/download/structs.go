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

type Job struct {
	ID         string
	URL        string
	Status     string
	CancelFunc context.CancelFunc
	OutputPath string
	Error      error
}

type RequestHeaders struct {
	Referer        string
	UserAgent      string
	AcceptLanguage string
	AcceptEncoding string
	Connection     string
}

type Options struct {
	SpeedLimit float64
	Out        string
	Parallel   int

	WantGroupFolder bool
	GroupFolder     string

	Referer   string
	UserAgent string
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
