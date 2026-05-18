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
