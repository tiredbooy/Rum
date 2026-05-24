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

func ConvertSizeToInt(size string) int64 {
	sizeStr := strings.TrimSpace(size)
	fileSize, _ := strconv.Atoi(sizeStr)

	return int64(fileSize)
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
