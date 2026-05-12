package download

import (
	"flag"
	"fmt"
	"log"
	"mime"
	"strings"
	"sync"

	"github.com/google/uuid"
	filesystem "swiftget.com/internal/pkg/file-system"
	"swiftget.com/internal/pkg/format"
	"swiftget.com/internal/pkg/utils"
)

var (
	resultChan chan DownloadResult
	jobs       = make(map[string]*Job)
	mu         sync.Mutex
)

func RunProgram(args []string) (map[string]*Job, []string, *Options, error) {
	// 1. Set up flags
	fs := flag.NewFlagSet("get", flag.ExitOnError)
	downloadDir := filesystem.GetOrCreateDirectory()

	var urls format.StringSlice
	fs.Func("url", "Download URLs", func(s string) error {
		urls = append(urls, s)
		return nil
	})

	out := fs.String("out", downloadDir, "Output directory")
	inputPath := fs.String("input", "", "Text file with URLs")
	parallel := fs.Int("p", 1, "Number of parallel downloads")
	limit := fs.Int("limit", 0, "Bandwidth limit (MB/s)")
	userAgent := fs.String("uA", "", "Custom User-Agent")
	referer := fs.String("rE", "", "Custom Referer")
	groupFolder := fs.String("group", "", "Create Group Folder")

	retry := fs.Int("retry", 3, "Max retries on failure")
	silent := fs.Bool("silent", false, "Suppress notifications")

	if err := fs.Parse(args); err != nil {
		return nil, nil, nil, fmt.Errorf("flag parse: %w", err)
	}

	var limitKB int = 0
	if *limit > 0 {
		limitKB = *limit * 1024
	}

	var wantGroup bool
	if *groupFolder != "" {
		wantGroup = true
	}

	// 2. Collect URLs from leftover args
	urls = append(urls, fs.Args()...)

	// 3. Prepare options
	opt := &Options{
		Out:             *out,
		Parallel:        *parallel,
		SpeedLimit:      limitKB,
		UserAgent:       *userAgent,
		Referer:         *referer,
		MaxRetries:      *retry,
		Silent:          *silent,
		WantGroupFolder: wantGroup,
		GroupFolder:     *groupFolder,
	}
	Opt = opt
	LoadOptions(opt)

	// 4. Read URLs from input file if provided
	if *inputPath != "" {
		fileURLs, err := filesystem.GetTxtUrls(*inputPath)
		if err != nil {
			return nil, nil, nil, fmt.Errorf("read input file: %w", err)
		}
		urls = append(urls, fileURLs...)

		// Prompt for group folder
		var want string
		fmt.Print("Do you want a Group Folder? (Y/N): ")
		fmt.Scanln(&want)
		want = strings.TrimSpace(strings.ToUpper(want))
		if want == "Y" {
			var name string
			fmt.Print("Enter folder name: ")
			fmt.Scanln(&name)
			name = strings.TrimSpace(name)
			if name == "" {
				name = "Downloads"
			}
			opt.WantGroupFolder = true
			opt.GroupFolder = name
		}
	}

	// 5. Prepare the Downloader (set a random user agent if none given)
	if *userAgent == "" {
		*userAgent = utils.GetRandomUserAgent()
	}
	downloader := NewDownloader(*userAgent, *referer)
	opt.Downloader = downloader

	// 6. Build final URL list (no duplicates)
	unique := make(map[string]bool)
	var finalURLs []string
	for _, u := range urls {
		if unique[u] {
			fmt.Printf("⚠️ Skipped duplicate: %s\n", u)
			continue
		}
		unique[u] = true
		finalURLs = append(finalURLs, u)
	}

	if len(finalURLs) == 0 {
		return nil, nil, nil, fmt.Errorf("at least one URL required")
	}

	// 7. Load saved jobs from disk (so we know which URLs already have a job)
	LoadJobsFromDisk()

	// 8. Find which URLs are NEW (not already in the jobs map)
	mu.Lock()
	existingURLs := make(map[string]bool)
	for _, j := range jobs {
		existingURLs[j.URL] = true
	}
	mu.Unlock()

	type newURL struct {
		idx int
		url string
	}
	var toHead []newURL
	for i, url := range finalURLs {
		if !existingURLs[url] {
			toHead = append(toHead, newURL{idx: i, url: url})
		}
	}

	// 9. Run HEAD requests for new URLs all at the same time (concurrent)
	//    We use a "semaphore" to limit how many goroutines run at once.
	maxConcurrent := 5
	if opt.Parallel > 1 {
		maxConcurrent = opt.Parallel
	}
	sem := make(chan struct{}, maxConcurrent)
	resultCh := make(chan HeadResult, len(toHead))

	for _, item := range toHead {
		sem <- struct{}{} // wait if we already have maxConcurrent goroutines running
		go func(n newURL) {
			defer func() { <-sem }() // release the slot when done
			info, err := downloader.HeadWithFallback(n.url)
			resultCh <- HeadResult{URL: n.url, FileInfo: info, Err: err}
		}(item)
	}

	// 10. Collect all results and build new Job objects
	newJobs := make(map[string]*Job)
	for i := 0; i < len(toHead); i++ {
		res := <-resultCh
		if res.Err != nil {
			log.Printf("⚠️ Failed to get file info for %s: %v", res.URL, res.Err)
			continue
		}

		// Determine the filename (same logic as before)
		var fileName string
		if res.FileInfo.ContentDisposition != "" {
			_, params, err := mime.ParseMediaType(res.FileInfo.ContentDisposition)
			if err == nil {
				fileName = params["filename"]
			}
		}
		if fileName == "" {
			fileName = format.ExtractFileNameFromURL(res.URL)
		}
		if fileName == "" {
			fileName = format.CleanFileName(res.URL)
		}
		if fileName == "" || fileName == "/" {
			fileName = "downloaded.file"
		}

		job := &Job{
			ID:           uuid.New().String(),
			URL:          res.URL,
			OutputPath:   opt.Out,
			FileName:     fileName,
			TotalSize:    utils.ConvertSizeToInt(res.FileInfo.ContentSize),
			ContentType:  res.FileInfo.ContentType,
			SupportRange: res.FileInfo.SupportsRange,
			Status:       "pending",
		}
		newJobs[job.ID] = job
	}

	// 11. Merge the new jobs into the global job map
	mu.Lock()
	for id, j := range newJobs {
		jobs[id] = j
	}
	mu.Unlock()

	// 12. Build the ordered slice of job IDs (same order as finalURLs)
	jobOrder := make([]string, 0, len(finalURLs))
	mu.Lock()
	for _, url := range finalURLs {
		for id, j := range jobs {
			if j.URL == url {
				jobOrder = append(jobOrder, id)
				break
			}
		}
	}
	mu.Unlock()

	SaveJobsToDisk()
	return jobs, jobOrder, opt, nil
}
