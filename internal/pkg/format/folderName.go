package format

import (
	"path"
	"strings"
)

func GetFolderName(contentType string) string {
	switch {
	case strings.HasPrefix(contentType, "application/x-rar-compressed"),
		strings.HasPrefix(contentType, "application/zip"),
		strings.HasPrefix(contentType, "application/x-7z-compressed"),
		strings.HasPrefix(contentType, "application/x-tar"),
		strings.HasPrefix(contentType, "application/gzip"):
		return "compressed"

	case strings.HasPrefix(contentType, "video/"):
		return "videos"
	case strings.HasPrefix(contentType, "application/vnd.microsoft.portable-executable"),
		strings.HasPrefix(contentType, "application/x-msdownload"),
		strings.HasPrefix(contentType, "application/x-executable"):
		return "programs"

	case strings.HasPrefix(contentType, "audio/"):
		return "audios"

	default:
		return "others"
	}
}

func CleanFileName(url string) string {
	urlArr := strings.Split(url, "?")
	fileName := path.Base(urlArr[0])
	return fileName
}
