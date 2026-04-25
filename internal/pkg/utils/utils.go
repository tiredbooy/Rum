package utils

import (
	"strings"
)

func UrlValidation(rawURL string) string {
	url := rawURL
	if !strings.HasPrefix(rawURL, "http://") && !strings.HasPrefix(rawURL, "https://") {
		url = "https://" + rawURL
	}

	return url
}
