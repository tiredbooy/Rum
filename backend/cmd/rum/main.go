package server

import (
	"fmt"
	"log"
	"os"

	"github.com/gen2brain/beeep"
	"github.com/tiredbooy/Rum/backend/internal/pkg/config"
	"github.com/tiredbooy/Rum/backend/internal/pkg/download"
	"github.com/tiredbooy/Rum/backend/internal/pkg/tui"
)

func main() {
	cfg := config.Load()

	var setting config.Setting
	err := setting.LoadSettingMetadata()
	if err != nil {
		log.Println("Error Opening setting: ", err.Error())
		return
	}
	opt := &download.Options{
		SpeedLimit: setting.SpeedLimitKB,
		Parallel:   setting.MaxParallel,
		Out:        setting.OutDir,
		MaxRetries: setting.MaxRetries,
		Silent:     setting.Silent,
	}

	opt.Downloader = download.NewDownloader("", "")

	download.LoadOptions(opt)

	args := os.Args[1:]

	if len(args) == 0 {
		printUsage()
		os.Exit(1)
	}

	download.InitLogFile()

	beeep.AppName = cfg.AppName

	sub := args[0]
	switch sub {
	case "get", "-g", "download":
		jobs, jobOrder, opt, err := download.RunProgram(args[1:])
		if err != nil {
			log.Println("Failed To run Program: ", err.Error())
			return
		}
		if jobs == nil && len(jobs) == 0 {
			return
		}
		tui.RunTUI(jobs, jobOrder, opt)
	case "version", "v", "-v", "--v":
		fmt.Printf("%s v%s ", cfg.AppName, cfg.Version)
	case "help", "--help", "-h", "--h":
		printUsage()
	default:
		jobs, jobOrder, opt, err := download.RunProgram(args)
		if err != nil {
			log.Println("Failed To run Program: ", err.Error())
			return
		}
		if jobs == nil && len(jobs) == 0 {
			return
		}
		tui.RunTUI(jobs, jobOrder, opt)
	}
}

func printUsage() {
	fmt.Println("Usage:")
	fmt.Println("  Rum get [flags] URL...")
	fmt.Println("  Rum version")
	fmt.Println("Run Server(API): ")
	fmt.Println("go run ./cmd/server")
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
