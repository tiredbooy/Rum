package download

type DownloadTask struct {
	Index    int
	URL      string
	Attempts int
	Options
}

type Options struct {
	SpeedLimit float64
	Out        string
	Parallel   int

	WantGroupFolder bool
	GroupFolder     string
}

type DownloadResult struct {
	URL     string
	Success bool
	Error   error
}
