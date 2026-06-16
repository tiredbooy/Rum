package main

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
		printUsage(cfg)
		os.Exit(1)
	}

	download.InitLogFile()

	beeep.AppName = cfg.AppName

	sub := args[0]
	switch sub {
	case "get", "-g", "download":
		runDownloads(cfg, args[1:])
	case "version", "v", "-v", "--v", "--version":
		printVersion(cfg)
	case "help", "--help", "-h", "--h":
		printUsage(cfg)
	default:
		// No recognized subcommand: treat the whole arg list as a `get`.
		// This keeps the old behaviour of `rum --url ...` working.
		runDownloads(cfg, args)
	}
}

// runDownloads parses flags, builds the jobs and starts the TUI.
func runDownloads(cfg *config.Config, args []string) {
	// Allow non-interactive operation (CI / scripts). When set, the engine
	// must not block on stdin prompts (e.g. the group-folder question).
	// See REQUESTED UPSTREAM CHANGES in the report: RunProgram should honour
	// the RUM_NONINTERACTIVE environment variable. We strip the user-facing
	// flags here and translate them into the env var so the rest of the
	// arguments are still valid for the engine's flag parser.
	args = applyNonInteractiveFlags(args)

	jobs, jobOrder, opt, err := download.RunProgram(args)
	if err != nil {
		log.Println("Failed To run Program: ", err.Error())
		return
	}
	if len(jobs) == 0 {
		return
	}
	tui.RunTUI(jobs, jobOrder, opt)
}

// applyNonInteractiveFlags scans for --yes / --non-interactive / -y, removes
// them from the argument slice and exports RUM_NONINTERACTIVE=1 so the engine
// (and any nested prompt) can detect a non-interactive run. Returns the
// filtered argument list.
func applyNonInteractiveFlags(args []string) []string {
	out := make([]string, 0, len(args))
	for _, a := range args {
		switch a {
		case "--yes", "-y", "--non-interactive", "--noninteractive":
			os.Setenv("RUM_NONINTERACTIVE", "1")
		default:
			out = append(out, a)
		}
	}
	// Honour an externally-set env var too (e.g. CI exporting it directly).
	if os.Getenv("RUM_NONINTERACTIVE") != "" {
		os.Setenv("RUM_NONINTERACTIVE", "1")
	}
	return out
}

func printVersion(cfg *config.Config) {
	fmt.Printf("%s v%s\n", cfg.AppName, cfg.Version)
}

func printUsage(cfg *config.Config) {
	fmt.Printf("%s v%s — a fast, parallel download manager\n\n", cfg.AppName, cfg.Version)
	fmt.Println("Usage:")
	fmt.Println("  rum get [flags] URL...")
	fmt.Println("  rum get --url URL [--url URL ...] [flags]")
	fmt.Println("  rum version")
	fmt.Println("  rum help")
	fmt.Println()
	fmt.Println("Commands:")
	fmt.Println("  get, download    Download one or more URLs")
	fmt.Println("  version          Print the version and exit")
	fmt.Println("  help             Show this help and exit")
	fmt.Println()
	fmt.Println("Flags (for `get`):")
	fmt.Println("  --url URL        Add a download URL (repeatable). Bare URLs also work.")
	fmt.Println("  --out DIR        Output directory")
	fmt.Println("  --input FILE     Text file with one URL per line")
	fmt.Println("  -p N             Number of parallel downloads (default 1)")
	fmt.Println("  --limit RATE     Bandwidth limit in MB/s (0 = unlimited)")
	fmt.Println("  --group NAME     Place all downloads into a group folder named NAME")
	fmt.Println("  --uA AGENT       Custom User-Agent")
	fmt.Println("  --rE REFERER     Custom Referer")
	fmt.Println("  --retry N        Max retries on failure (default 3)")
	fmt.Println("  --silent         Suppress desktop notifications and sounds")
	fmt.Println("  --yes, -y        Non-interactive: never prompt on stdin (for CI/scripts)")
	fmt.Println("                   (also enabled by setting RUM_NONINTERACTIVE=1)")
	fmt.Println()
	fmt.Println("Examples:")
	fmt.Println("  rum get https://example.com/file.zip --out ./downloads -p 4")
	fmt.Println("  rum get --input urls.txt --out ./downloads --group myset --yes")
	fmt.Println("  rum get --url https://a/x.iso --url https://b/y.iso -p 2")
	fmt.Println()
	fmt.Println("Run the HTTP API server instead:")
	fmt.Println("  go run ./cmd/server")
}
