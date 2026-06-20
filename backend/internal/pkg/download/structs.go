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

	// VerifyIntegrity, when true, makes a finished segmented download compute a
	// full-file SHA-256 and store it in the sidecar/job so a later "verify" can
	// be performed without the server (server-free re-verification). Structural
	// verification (segment completeness + on-disk size, which catches sparse
	// holes) always runs regardless of this flag; this only controls whether the
	// extra full-file hash is computed and persisted. Default false keeps the
	// fast path for callers that have not opted into it.
	VerifyIntegrity bool

	// RetryBackoffSec is the base exponential-backoff delay (seconds) used by the
	// engine retry config. 0 means "use the engine default" (defaultRetryBaseDelay).
	// Threaded from config.Setting.RetryBackoffSec (clamped to [1,60] there).
	RetryBackoffSec int

	// TempDir, when non-empty, is where the in-progress file (and its .rumparts
	// resume sidecar) are written; the finished file is moved to Out on
	// completion. Empty = write the in-progress file next to the final output
	// (the legacy behavior). Threaded from config.Setting.TempDir.
	TempDir string

	// KeepPartialOnFailure, when true, preserves on-disk partial data + the resume
	// sidecar when a download FAILS (so it can be resumed/repaired). It does not
	// affect explicit deletes, which always remove partials. Threaded from
	// config.Setting.KeepPartialOnFailure.
	KeepPartialOnFailure bool

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
