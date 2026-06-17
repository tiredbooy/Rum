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

	// Checksum, when non-empty, is verified against the assembled file after a
	// successful download. ChecksumAlgo selects the hash ("sha256" or "md5").
	// Empty Checksum skips verification (VerifyChecksum is then a no-op).
	Checksum     string
	ChecksumAlgo string

	// StartAt, when non-zero, is the earliest time a scheduled job should begin.
	// The schedule controller starts due jobs; a zero value means start
	// immediately (the default).
	StartAt time.Time

	// Categorize enables auto-organize on the finalize path: a finished file is
	// moved into a category destination based on its extension. Category, when
	// non-empty, forces a specific category instead of auto-detecting.
	Categorize bool
	Category   string

	// Governor, when set, is a process-wide shared bandwidth cap applied to this
	// download (and others sharing the same governor). When nil, the engine falls
	// back to the per-download limiter built from SpeedLimit / the global setting,
	// so CLI/engine callers are unaffected.
	Governor *SpeedGovernor

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
