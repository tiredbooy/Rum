package download

import (
	"log"
	"net/http"
)

func GetFile(url string) (*http.Request, error) {
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}

	return req, nil
}

func GetHeader(url string) (*http.Response, error) {
	req, err := http.NewRequest("HEAD", url, nil)
	if err != nil {
		log.Printf("Failed to create HEAD request for %s: %v", url, err)
		return nil, err
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		log.Printf("Failed to perform HEAD request for %s: %v", url, err)
		return nil, err
	}

	return resp, nil
}
