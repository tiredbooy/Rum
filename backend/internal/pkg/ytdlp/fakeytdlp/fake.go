// Package fakeytdlp is a stand-in yt-dlp for tests. A test binary points the
// code under test at itself (os.Args[0]) with Env set; its TestMain then calls
// Main instead of running tests. Behaviour is chosen with RUM_FAKE_MODE.
package fakeytdlp

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"
)

const (
	// Env, when non-empty in the process environment, makes TestMain act as yt-dlp.
	Env = "RUM_FAKE_YTDLP"
	// ModeEnv selects behaviour: "" (works), "unavailable", "slow", "fail".
	ModeEnv = "RUM_FAKE_MODE"
	// ArgsEnv names a file the fake appends its argv to, one arg per line.
	ArgsEnv = "RUM_FAKE_ARGS"
)

// InfoJSON is what `-J` prints: one progressive 360p, DASH 1080/720/480 video
// streams and an m4a audio stream.
const InfoJSON = `{"id":"abc123","title":"Test Video: 100% real/fake","extractor_key":"Youtube","duration":93.5,
"formats":[
{"format_id":"18","ext":"mp4","height":360,"vcodec":"avc1","acodec":"mp4a","filesize":3000000,"tbr":500},
{"format_id":"137","ext":"mp4","height":1080,"vcodec":"avc1","acodec":"none","filesize":40000000,"tbr":4000},
{"format_id":"136","ext":"mp4","height":720,"vcodec":"avc1","acodec":"none","filesize":20000000,"tbr":2000},
{"format_id":"135","ext":"mp4","height":480,"vcodec":"avc1","acodec":"none","filesize":10000000,"tbr":1000},
{"format_id":"140","ext":"m4a","height":null,"vcodec":"none","acodec":"mp4a","filesize":1500000,"tbr":128}
]}`

// Main runs the fake and returns the exit code.
func Main(args []string, stdout, stderr io.Writer) int {
	if f := os.Getenv(ArgsEnv); f != "" {
		if fh, err := os.OpenFile(f, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o600); err == nil {
			fmt.Fprintln(fh, strings.Join(args, "\n"))
			fh.Close()
		}
	}
	mode := os.Getenv(ModeEnv)
	has := func(s string) bool {
		for _, a := range args {
			if a == s {
				return true
			}
		}
		return false
	}
	switch {
	case has("--version"):
		fmt.Fprintln(stdout, "2025.01.01")
		return 0
	case has("-U"):
		fmt.Fprintln(stdout, "yt-dlp is up to date")
		return 0
	}
	if mode == "unavailable" {
		fmt.Fprintln(stderr, "ERROR: [youtube] abc123: Video unavailable")
		return 1
	}
	if mode == "fail" {
		fmt.Fprintln(stderr, "ERROR: something odd happened token=secret")
		return 1
	}
	if has("-J") {
		fmt.Fprintln(stdout, InfoJSON)
		return 0
	}
	return download(args, stdout, mode)
}

func download(args []string, stdout io.Writer, mode string) int {
	var tmpl string
	for i, a := range args {
		if a == "-o" && i+1 < len(args) {
			tmpl = args[i+1]
		}
	}
	if tmpl == "" {
		return 2
	}
	final := strings.ReplaceAll(tmpl, "%(ext)s", "mp4")
	part := final + ".part"
	if _, err := os.Stat(part); err == nil {
		fmt.Fprintln(stdout, "[download] Resuming download at byte 2500000")
	}
	if err := os.MkdirAll(filepath.Dir(final), 0o755); err != nil {
		return 1
	}
	if err := os.WriteFile(part, []byte("partial"), 0o644); err != nil {
		return 1
	}
	const total = 10000000
	delay := time.Millisecond
	if mode == "slow" {
		delay = 50 * time.Millisecond
	}
	// Two streams, like a merged video+audio download: each counts from zero.
	for stream := 0; stream < 2; stream++ {
		for pct := 10; pct <= 100; pct += 10 {
			got := total * pct / 100
			status := "downloading"
			if pct == 100 {
				status = "finished"
			}
			fmt.Fprintf(stdout, "RUMP|%s|%d|%d|NA|%d|%d\n", status, got, total, 1000000, (100-pct)/10)
			time.Sleep(delay)
		}
	}
	if mode == "slow" {
		time.Sleep(time.Hour) // never reached when the test pauses/cancels
	}
	if err := os.Rename(part, final); err != nil {
		return 1
	}
	fmt.Fprintf(stdout, "RUMFILE|%s\n", final)
	return 0
}
