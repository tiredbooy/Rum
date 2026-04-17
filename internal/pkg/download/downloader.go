package download

import (
	"flag"
	"fmt"
	"log"
	"sync"

	filesystem "swiftget.com/internal/pkg/file-system"
	"swiftget.com/internal/pkg/format"
)

var resultChan chan DownloadResult

func RunProgram(args []string) {
	var (
		wg sync.WaitGroup
		mu sync.Mutex
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

	// resume := fs.Bool("r", true, "Resume Download")
	// bandewith := fs.Int("-b", 1000, "Network Bandewitch")
	// userAgent := fs.String("-u", "", "User Agent")

	if err := fs.Parse(args); err != nil {
		log.Printf("Error parsing flags: %v\n", err)
		fs.Usage()
		return
	}

	rest := fs.Args()
	urls = append(urls, rest...)

	if *inputPath != "" {
		txtFileURLs, err := filesystem.GetTxtUrls(*inputPath)
		if err != nil {
			log.Printf("ERROR reading input file %s: %v\n", *inputPath, err.Error())
			return
		}

		urls = append(urls, txtFileURLs...)

	}

	if len(urls) == 0 {
		fmt.Println("URLS: ", urls)
		fmt.Println("Error: Atleast one -url is Required")
		fs.Usage()
		return
	}

	fmt.Println("Download Started...")
	totalURLs := len(urls)
	resultChan = make(chan DownloadResult, totalURLs)
	go collectResult(totalURLs)
	fmt.Printf("Starting download of %d URLs to %s with %d parallel workers...\n", totalURLs, *out, *parallel)

	semphoreChan := make(chan struct{}, *parallel)

	for _, url := range urls {
		semphoreChan <- struct{}{}
		wg.Add(1)

		go func(currentURL string) {
			defer func() {
				<-semphoreChan
				wg.Done()
			}()

			opt := Options{
				URL:      currentURL,
				Out:      *out,
				Parallel: *parallel,
			}

			mu.Lock()
			// StartDownload(opt)
			DownloadWorker(opt)
			mu.Unlock()
		}(url)
	}

	// wg.Add(1)

	// go func() {
	// 	defer wg.Done()
	// 	for _, url := range urls {
	// 		wg.Add(*parallel)
	// 		go func() {
	// 			defer wg.Done()
	// 			mu.Lock()
	// 			opt := Options{
	// 				URL:      url,
	// 				Out:      *out,
	// 				Parallel: *parallel,
	// 			}
	// 			DownloadWorker(opt)
	// 			mu.Unlock()
	// 		}()
	// 	}
	// }()

	// wg.Wait()

	fmt.Printf("\rDownload have been completed\n")

	// fmt.Println("URL :", *url)
	// opt := download.Options{
	// 	URL: *url,
	// 	Out: downloadDir,
	// }

}
