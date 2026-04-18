package download

import (
	"errors"
	"log"
)

func DownloadWorker(task DownloadTask) {
	if task.URL == "" {
		resultChan <- DownloadResult{URL: task.URL, Success: false, Error: errors.New("Invalid URL provided")}
		return
	}

	err := DownloadSingleFile(*Opt, task.URL)
	if err != nil {
		log.Printf("Error downloading %s: %v\n", task.URL, err.Error())
		resultChan <- DownloadResult{URL: task.URL, Success: false, Error: err}
		return
	}

	log.Printf("\rSuccessfully Downloaded: %s\n", task.URL)
	resultChan <- DownloadResult{URL: task.URL, Success: true, Error: nil}

}
