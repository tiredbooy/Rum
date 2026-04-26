package download

import (
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/gen2brain/beeep"
	"github.com/google/uuid"
	filesystem "swiftget.com/internal/pkg/file-system"
	"swiftget.com/internal/pkg/format"
)

var (
	resultChan chan DownloadResult
	jobs       = make(map[string]*Job)
	mu         sync.Mutex
)

func RunProgram(args []string) (map[string]*Job, *Options) {
	var (
		wantGroupFolder string
		groupFolderName string
	)
	beeep.AppName = "Rum"
	fs := flag.NewFlagSet("get", flag.ExitOnError)
	downloadDir := filesystem.GetOrCreateDirectory()

	configPath := GetJobsFilePath()
	testFile := configPath + ".test"
	if err := os.WriteFile(testFile, []byte("test"), 0644); err != nil {
		fmt.Printf("⚠️ WARNING: Cannot write to config directory: %v\n", err)
		fmt.Printf("   Tried to write: %s\n", testFile)
	} else {
		os.Remove(testFile)
		fmt.Printf("✓ Config directory writable: %s\n", filepath.Dir(configPath))
	}

	var urls format.StringSlice
	fs.Func("url", "Download URLs", func(flagValue string) error {
		urls = append(urls, flagValue)
		return nil
	})

	out := fs.String("out", downloadDir, "Output Directory")
	inputPath := fs.String("input", "", "Input URLs Text File")
	parallel := fs.Int("p", 1, "Parallel Download")
	limit := fs.Float64("limit", 0, "Network bandwidth limit (MB/s)")
	userAgent := fs.String("uA", "", "User Agent")
	referer := fs.String("rE", "", "Referer")

	if err := fs.Parse(args); err != nil {
		log.Printf("Error parsing flags: %v\n", err)
		fs.Usage()
		return nil, nil
	}

	rest := fs.Args()
	urls = append(urls, rest...)

	opt := &Options{
		Out:        *out,
		Parallel:   *parallel,
		SpeedLimit: *limit,
		UserAgent:  *userAgent,
		Referer:    *referer,
	}
	Opt = opt

	// Handle input file if provided
	if *inputPath != "" {
		txtFileURLs, err := filesystem.GetTxtUrls(*inputPath)
		if err != nil {
			log.Printf("ERROR reading input file %s: %v\n", *inputPath, err.Error())
			return nil, nil
		}
		urls = append(urls, txtFileURLs...)

		for {
			fmt.Print("Do you want a Group Folder? (Y,N): ")
			fmt.Scanln(&wantGroupFolder)
			wantGroupFolder = strings.TrimSpace(strings.ToUpper(wantGroupFolder))
			if wantGroupFolder == "Y" || wantGroupFolder == "N" {
				break
			}
			fmt.Println("Please enter only Y or N")
		}

		if wantGroupFolder == "Y" {
			for {
				fmt.Print("Enter the folder name: ")
				fmt.Scanln(&groupFolderName)
				groupFolderName = strings.TrimSpace(groupFolderName)
				if groupFolderName != "" {
					break
				}
				fmt.Println("Folder name cannot be empty")
			}
			opt.WantGroupFolder = true
			opt.GroupFolder = groupFolderName
		}
	}

	if len(urls) == 0 {
		fmt.Println("Error: at least one URL is required")
		fs.Usage()
		return nil, nil
	}

	LoadOptions(opt)

	LoadJobsFromDisk()

	for _, url := range urls {
		job := &Job{
			ID:         uuid.New().String(),
			URL:        url,
			OutputPath: opt.Out,
			Status:     "pending",
		}
		mu.Lock()
		jobs[job.ID] = job
		mu.Unlock()
	}

	SaveJobsToDisk()

	return jobs, opt
}
