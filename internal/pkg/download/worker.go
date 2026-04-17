package download

import (
	"errors"
	"log"
)

func DownloadWorker(opt Options) {
	if opt.URL == "" {
		resultChan <- DownloadResult{URL: opt.URL, Success: false, Error: errors.New("Invalid URL provided")}
		return
	}

	err := DownloadSingleFile(opt)
	if err != nil {
		log.Printf("Error downloading %s: %v\n", opt.URL, err)
		resultChan <- DownloadResult{URL: opt.URL, Success: false, Error: err}
		return
	}

	log.Printf("\rSuccessfully Downloaded: %s\n", opt.URL)
	resultChan <- DownloadResult{URL: opt.URL, Success: true, Error: nil}

}
