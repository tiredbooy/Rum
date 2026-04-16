package main

import (
	"flag"
	"fmt"
	"log"
	"os"

	"swiftget.com/internal/pkg/download"
	filesystem "swiftget.com/internal/pkg/file-system"
)

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
	fs := flag.NewFlagSet("get", flag.ExitOnError)
	downloadDir := filesystem.GetOrCreateDirectory()

	// out := fs.String("out", downloadDir, "Output Directory")
	url := fs.String("url", "", "Download File URL")
	
	// resume := fs.Bool("r", true, "Resume Download")
	// parallel := fs.Int("-p", 1, "Parallel Download")
	// bandewith := fs.Int("-b", 1000, "Network Bandewitch")
	// userAgent := fs.String("-u", "", "User Agent")

	_ = fs.Parse(args)

	fmt.Println("URL :", *url)
	opt := download.Options{
		URL: *url,
		Out: downloadDir,
	}

	log.Print(opt)

	// fmt.Println("Download Started...")
	// err := download.StartDownload(opt)
	// if err != nil {
	// 	fmt.Println("Download Failed", err)
	// 	return
	// }

	// fmt.Printf("\rDownload have been completed\n")

}

func printUsage() {
	fmt.Println("Usage:")
	fmt.Println("  switftget URL [flags]")
	fmt.Println("  switftget version")
	fmt.Println()
	fmt.Println("Examples:")
	fmt.Println("  switftget URL --out [OUTPUT PATH] --p 100 --b 1200(KB)")
}
