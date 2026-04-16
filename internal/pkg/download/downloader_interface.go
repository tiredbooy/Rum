package download

// import "golang.org/x/sync/errgroup"

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
	URL string
	Out string
}