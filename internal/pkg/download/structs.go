package download

import (
	"context"
	"time"
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
	ID            string             `json:"id"`
	URL           string             `json:"url"`
	Status        string             `json:"status"`
	CancelFunc    context.CancelFunc `json:"-"`
	OutputPath    string             `json:"output_path"`
	TotalSize     int64              `json:"total_size"`
	Error         error              `json:"error"`
	Downloaded    int64              `json:"downloaded"`
	Speed         float64            `json:"speed"`
	RemainingTime time.Duration      `json:"remaining_time"`
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
