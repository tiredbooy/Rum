package download

import (
	"fmt"
	"log"
)

var retryChan chan DownloadTask

func GatherFailedURLs() {
	var count int
	for result := range resultChan {
		if !result.Success {
			task := DownloadTask{
				URL:      result.URL,
				Attempts: +1,
			}

			log.Println("FOUND ERROR")
			retryChan <- task
			// tasks = append(tasks, task)
			count++
		}
	}

	fmt.Printf("\r Found %v Failed Files \n", count)
}

// func RetryDownload(retryCount int, timout float64) {
// 	// tasks := GatherFailedURLs()
// 	// if len(tasks) <= 0 {
// 	// 	log.Println("Nothing To Retry")
// 	// 	return
// 	// }

// 	fmt.Println("Retrying Download...")
// 	for task := range retryChan {
// 		DownloadWorker(task)
// 	}
// }
