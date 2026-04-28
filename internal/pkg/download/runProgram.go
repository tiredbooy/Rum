package download

import (
	"flag"
	"fmt"
	"strings"
	"sync"

	// "github.com/gen2brain/beeep"
	"github.com/google/uuid"
	filesystem "swiftget.com/internal/pkg/file-system"
	"swiftget.com/internal/pkg/format"
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
	limit := fs.Float64("limit", 0, "Bandwidth limit (MB/s)")
	userAgent := fs.String("uA", "", "Custom User-Agent")
	referer := fs.String("rE", "", "Custom Referer")
	// New flags (see Section 3)
	retry := fs.Int("retry", 3, "Max retries on failure")
	silent := fs.Bool("silent", false, "Suppress notifications")

	if err := fs.Parse(args); err != nil {
		return nil, nil, nil, fmt.Errorf("flag parse: %w", err)
	}

	// 2. Collect URLs from leftover args
	urls = append(urls, fs.Args()...)

	// 3. Prepare options and load persisted jobs
	opt := &Options{
		Out:        *out,
		Parallel:   *parallel,
		SpeedLimit: *limit,
		UserAgent:  *userAgent,
		Referer:    *referer,
		MaxRetries: *retry,
		Silent:     *silent,
	}
	Opt = opt
	LoadOptions(opt)

	// 4. Read URLs from input file
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

	// 5. Build final URL list (deduplicated)
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

	// 6. Load previous jobs from disk and merge new URLs
	LoadJobsFromDisk()
	mu.Lock()
	for _, url := range finalURLs {
		exists := false
		for _, j := range jobs {
			if j.URL == url {
				exists = true
				break
			}
		}
		if !exists {
			job := &Job{
				ID:         uuid.New().String(),
				URL:        url,
				OutputPath: opt.Out,
				Status:     "pending",
			}
			jobs[job.ID] = job
		}
	}
	mu.Unlock()

	// 7. Create ordered ID slice matching finalURLs order
	jobOrder := make([]string, 0, len(finalURLs))
	for _, url := range finalURLs {
		for id, j := range jobs {
			if j.URL == url {
				jobOrder = append(jobOrder, id)
				break
			}
		}
	}

	SaveJobsToDisk()
	return jobs, jobOrder, opt, nil
}
