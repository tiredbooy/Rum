package main

import (
	"fmt"
	"os"

	"swiftget.com/internal/pkg/download"
	"swiftget.com/internal/pkg/tui"
)

func main() {
	args := os.Args[1:]
	if len(args) == 0 {
		printUsage()
		os.Exit(1)
	}

	download.InitLogFile()

	sub := args[0]
	switch sub {
	case "get":
		jobs, opt := download.RunProgram(args[1:])
		if jobs == nil || len(jobs) == 0 {
			return
		}
		tui.RunTUI(jobs, opt)
	case "version":
		fmt.Println("Rum v0.1.0")
	case "help", "--help", "-h":
		printUsage()
	default:
		// Assume first argument is a URL
		jobs, opt := download.RunProgram(args)
		if jobs == nil || len(jobs) == 0 {
			return
		}
		tui.RunTUI(jobs, opt)
	}
}

func printUsage() {
	fmt.Println("Usage:")
	fmt.Println("  Rum get [flags] URL...")
	fmt.Println("  Rum version")
	fmt.Println()
	fmt.Println("Examples:")
	fmt.Println("  Rum get https://example.com/file.zip --out ./downloads -p 4")
	fmt.Println("  Rum get --input urls.txt --out ./downloads")
	fmt.Println()
	fmt.Println("Flags:")
	fmt.Println("  --out DIR      Output directory")
	fmt.Println("  --input FILE   Text file with URLs")
	fmt.Println("  -p N           Parallel downloads")
	fmt.Println("  --limit RATE   Bandwidth limit (MB/s)")
	fmt.Println("  --uA AGENT     User-Agent")
	fmt.Println("  --rE REFERER   Referer")
}
