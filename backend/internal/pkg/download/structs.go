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

	// Connections is the number of concurrent connections (segments) to use
	// for a single download when the server supports HTTP Range requests and
	// the total size is known and large enough. 0 or 1 => single-stream
	// download (the legacy behavior). Defaults are applied by the engine when
	// unset.
	Connections int

	Downloader *Downloader
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

type HeadResult struct {
	Index    int 
	URL      string
	FileInfo *HeaderInfo
	Err      error
}
