package utils

import (
	"fmt"
	"os/exec"
	"runtime"
	"strconv"
	"strings"
)

func UrlValidation(rawURL string) string {
	url := rawURL
	if !strings.HasPrefix(rawURL, "http://") && !strings.HasPrefix(rawURL, "https://") {
		url = "https://" + rawURL
	}

	return url
}

// ConvertSizeToInt parses a Content-Length-style byte count. It is deliberately
// lenient: it returns 0 for empty/invalid input (so a bad header is treated as
// "unknown size") and ignores any non-digit suffix/prefix noise such as "1234
// bytes". Negative values are clamped to 0.
func ConvertSizeToInt(size string) int64 {
	s := strings.TrimSpace(size)
	if s == "" {
		return 0
	}

	// Fast path: a clean integer.
	if n, err := strconv.ParseInt(s, 10, 64); err == nil {
		if n < 0 {
			return 0
		}
		return n
	}

	// Lenient path: pull the leading run of digits (handles "12345 bytes",
	// "+12345", surrounding whitespace, etc.).
	start := 0
	if s[0] == '+' || s[0] == '-' {
		start = 1
	}
	end := start
	for end < len(s) && s[end] >= '0' && s[end] <= '9' {
		end++
	}
	if end == start {
		return 0
	}
	n, err := strconv.ParseInt(s[start:end], 10, 64)
	if err != nil || s[0] == '-' {
		return 0
	}
	return n
}

func GetProgress(downloaded, totalSize int64) int {
	if totalSize <= 0 {
		return 0
	}

	return int((downloaded * 100) / totalSize)
}

func IsTerminal(status string) bool {
	switch status {
	case "completed", "failed", "cancelled":
		return true
	default:
		return false
	}
}

func ShutdownPC() error {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "windows":
		cmd = exec.Command("shutdown", "/s", "/t", "0")
	case "darwin": // macOS
		cmd = exec.Command("osascript", "-e", `tell app "System Events" to shut down`)
	case "linux":
		cmd = exec.Command("shutdown", "-h", "now")
	default:
		return fmt.Errorf("unsupported OS")
	}
	return cmd.Run()
}

func SleepPC() error {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "windows":
		cmd = exec.Command("rundll32.exe", "powrprof.dll,SetSuspendState", "0", "1", "0")
	case "darwin": // macOS
		cmd = exec.Command("pmset", "sleepnow")
	case "linux":
		cmd = exec.Command("systemctl", "suspend")
	default:
		return fmt.Errorf("unsupported OS")
	}
	return cmd.Run()
}
