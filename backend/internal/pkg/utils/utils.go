package utils

import (
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
