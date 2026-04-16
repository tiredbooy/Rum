package main

import (
	"flag"
	"fmt"
	"os"
	"sync"

	"swiftget.com/internal/pkg/download"
	filesystem "swiftget.com/internal/pkg/file-system"
	"swiftget.com/internal/pkg/format"
)

// var (
// 	wg sync.WaitGroup
// 	mu sync.Mutex
// )

func main() {
	if len(os.Args) < 2 {
		runProgram(os.Args[1:])
		return
	}

	sub := os.Args[1]
	switch sub {
	case "get":
		runProgram(os.Args[1:])
	case "version":
		fmt.Println("SwitftGet v0.0.1")
	case "help":
		printUsage()
	default:
		runProgram(os.Args[1:])
		os.Exit(2)
		return
	}
}

func runProgram(args []string) {
	var (
		wg sync.WaitGroup
		mu sync.Mutex
	)
	fs := flag.NewFlagSet("get", flag.ExitOnError)
	downloadDir := filesystem.GetOrCreateDirectory()

	// url := fs.String("url", "", "Download File URL")
	var urls format.StringSlice
	// fs.Var(&urls, "url", "Download File URL (can be specified multipile times)")
	fs.Func("url", "Download URLs", func(flagValue string) error {
		urls = append(urls, flagValue)
		return nil
	})
	out := fs.String("out", downloadDir, "Output Directory")

	// resume := fs.Bool("r", true, "Resume Download")
	parallel := fs.Int("p", 1, "Parallel Download")
	// bandewith := fs.Int("-b", 1000, "Network Bandewitch")
	// userAgent := fs.String("-u", "", "User Agent")

	fs.Parse(args)

	rest := fs.Args()
	urls = append(urls, rest...)

	if len(urls) == 0 {
		fmt.Println("URLS: ", urls)
		fmt.Println("Error: Atleast one -url is Required")
		fs.Usage()
		return
	}

	fmt.Println("Download Started...")
	// wg.Add(1)
	// for _, url := range urls {
	// 	wg.Add(1)
	// 	go func() {
	// 		defer func() {
	// 			wg.Done()
	// 			mu.Unlock()
	// 		}()
	// 		log.Println("URL: ", url)
	// 		mu.Lock()
	// 		opt := download.Options{
	// 			URL:      url,
	// 			Out:      *out,
	// 			Parallel: *parallel,
	// 		}

	// 		go download.StartDownload(opt)

	// 		// wg.Wait()
	// 		// if err != nil {
	// 		// 	fmt.Println("Download Failed", err)
	// 		// 	return
	// 		// }
	// 	}()
	// }

	// for _, url := range urls {
	// 	wg.Add(1)
	// 	mu.Lock()
	// 	go func(url string) {
	// 		defer mu.Unlock()
	// 		defer wg.Done()
	// 		opt := download.Options{
	// 			URL:      url,
	// 			Out:      *out,
	// 			Parallel: *parallel,
	// 		}
	// 		download.StartDownload(opt)
	// 	}(url)
	// }

	wg.Add(1)

	go func() {
		defer wg.Done()
		for _, url := range urls {
			fmt.Println("URL: ", url)
			wg.Add(1)
			go func() {
				defer wg.Done()
				mu.Lock()
				opt := download.Options{
					URL:      url,
					Out:      *out,
					Parallel: *parallel,
				}
				download.DownloadWorker(opt)
				mu.Unlock()
			}()
		}
	}()

	wg.Wait()

	fmt.Printf("\rDownload have been completed\n")

	// fmt.Println("URL :", *url)
	// opt := download.Options{
	// 	URL: *url,
	// 	Out: downloadDir,
	// }

}

func printUsage() {
	fmt.Println("Usage:")
	fmt.Println("  switftget URL [flags]")
	fmt.Println("  switftget version")
	fmt.Println()
	fmt.Println("Examples:")
	fmt.Println("  switftget URL --out [OUTPUT PATH] --p 100 --b 1200(KB)")
}
