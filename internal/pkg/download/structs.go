package download

type DownloaderInterface interface {
	Download() error
	Cancel()
	// GetProgress() *Progress
}

type ParallelDownloader struct {
	// g         *errgroup.Group
	workers   int
	chunkSize int
}

type Options struct {
	URL      string
	Out      string
	Parallel int
}

type DownloadResult struct {
	URL     string
	Success bool
	Error   error
}
