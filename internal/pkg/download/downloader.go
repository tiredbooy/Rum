package download

import (
	"flag"
	"fmt"
	"log"
	"strings"
	"sync"

	filesystem "swiftget.com/internal/pkg/file-system"
	"swiftget.com/internal/pkg/format"
)

var resultChan chan DownloadResult

func RunProgram(args []string) {
	var (
		wg              sync.WaitGroup
		mu              sync.Mutex
		wantGroupFolder string
		groupFolderName string
	)
	fs := flag.NewFlagSet("get", flag.ExitOnError)
	downloadDir := filesystem.GetOrCreateDirectory()

	var urls format.StringSlice
	fs.Func("url", "Download URLs", func(flagValue string) error {
		urls = append(urls, flagValue)
		return nil
	})
	out := fs.String("out", downloadDir, "Output Directory")
	inputPath := fs.String("input", "", "Input URLs Text File ")
	parallel := fs.Int("p", 1, "Parallel Download")
	Limit := fs.Float64("limit", 0, "Network Bandewitch")
	// bandewith := fs.Int("-b", 1000, "Network Bandewitch")

	// resume := fs.Bool("r", true, "Resume Download")
	// userAgent := fs.String("-u", "", "User Agent")

	if err := fs.Parse(args); err != nil {
		log.Printf("Error parsing flags: %v\n", err)
		fs.Usage()
		return
	}

	rest := fs.Args()
	urls = append(urls, rest...)

	opt := Options{
		Out:        *out,
		Parallel:   *parallel,
		SpeedLimit: *Limit,
	}

	if *inputPath != "" {

		txtFileURLs, err := filesystem.GetTxtUrls(*inputPath)
		if err != nil {
			log.Printf("ERROR reading input file %s: %v\n", *inputPath, err.Error())
			return
		}

		urls = append(urls, txtFileURLs...)

		for {
			fmt.Print("Do You want a Group Folder? (Y,N): ")
			fmt.Scanln(&wantGroupFolder)
			wantGroupFolder = strings.TrimSpace(strings.ToUpper(wantGroupFolder))

			if wantGroupFolder == "Y" || wantGroupFolder == "N" {
				break
			}

			fmt.Println("Please Enter Only Y or N")
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
		fmt.Println("URLS: ", urls)
		fmt.Println("Error: Atleast one -url is Required")
		fs.Usage()
		return
	}

	LoadOptions(opt)

	fmt.Println("Download Started...")
	totalURLs := len(urls)
	resultChan = make(chan DownloadResult, totalURLs)
	go collectResult(totalURLs)
	fmt.Printf("Starting download of %d URLs to %s with %d parallel workers...\n", totalURLs, *out, *parallel)

	semphoreChan := make(chan struct{}, *parallel)

	for i, url := range urls {
		semphoreChan <- struct{}{}
		wg.Add(1)

		go func(currentURL string) {
			defer func() {
				<-semphoreChan
				wg.Done()
			}()

			task := DownloadTask{
				Index:    i,
				URL:      url,
				Attempts: 1,
			}

			mu.Lock()
			DownloadWorker(task)
			mu.Unlock()
		}(url)
	}

	GatherFailedURLs()

	fmt.Printf("\rDownloads have been completed\n")
}
