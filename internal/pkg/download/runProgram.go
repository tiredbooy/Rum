package download

import (
	"flag"
	"fmt"
	"strings"

	filesystem "github.com/tiredbooy/Rum/internal/pkg/file-system"
	"github.com/tiredbooy/Rum/internal/pkg/format"
	"github.com/tiredbooy/Rum/internal/pkg/utils"
)

func RunProgram(args []string) (map[string]*Job, []string, *Options, error) {
	// ---------- 1. Parse flags ----------
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

	var limitKB int
	if *limit > 0 {
		limitKB = *limit * 1024
	}

	// Collect URLs from leftover args
	urls = append(urls, fs.Args()...)

	// Build Options
	opt := &Options{
		Out:             *out,
		Parallel:        *parallel,
		SpeedLimit:      limitKB,
		UserAgent:       *userAgent,
		Referer:         *referer,
		MaxRetries:      *retry,
		Silent:          *silent,
		WantGroupFolder: *groupFolder != "",
		GroupFolder:     *groupFolder,
	}
	Opt = opt // keep global if needed elsewhere
	LoadOptions(opt)

	// ---------- 2. Handle input file (CLI‑specific) ----------
	if *inputPath != "" {
		fileURLs, err := filesystem.GetTxtUrls(*inputPath)
		if err != nil {
			return nil, nil, nil, fmt.Errorf("read input file: %w", err)
		}
		urls = append(urls, fileURLs...)

		// Interactive group folder prompt
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

	if opt.UserAgent == "" {
		opt.UserAgent = utils.GetRandomUserAgent()
	}
	// Create the HTTP downloader
	opt.Downloader = NewDownloader(opt.UserAgent, opt.Referer)

	// ---------- 3. Create the JobManager ----------
	manager := NewJobManager(opt)

	// ---------- 4. Deduplicate URLs ----------
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

	// ---------- 5. Create jobs (HEAD requests + job objects) ----------
	newJobs, err := manager.CreateJobsFromURLs(finalURLs)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("create jobs: %w", err)
	}
	_ = newJobs

	// ---------- 6. Build the ordered job ID slice (same order as finalURLs) ----------
	jobOrder := make([]string, 0, len(finalURLs))
	for _, url := range finalURLs {
		jobID, ok := manager.GetJobIDByURL(url)
		if !ok {
			return nil, nil, nil, fmt.Errorf("internal error: no job for URL %s", url)
		}
		jobOrder = append(jobOrder, jobID)
	}

	// ---------- 7. Return all jobs as a map (for compatibility with existing TUI) ----------
	allJobs := make(map[string]*Job)
	for _, job := range manager.GetAllJobs() {
		allJobs[job.ID] = job
	}

	return allJobs, jobOrder, opt, nil
}
