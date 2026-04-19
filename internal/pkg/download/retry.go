package download

import (
	"fmt"
	"log"
)

var retryChan chan DownloadTask

func GatherFailedURLs() int {
	var count int
	for result := range resultChan {
		if !result.Success {
			task := DownloadTask{
				URL:      result.URL,
				Attempts: +1,
			}

			retryChan <- task
			// tasks = append(tasks, task)
			count++

			log.Printf("Failed Count: %v", count)

		}
	}

	close(resultChan)

	fmt.Printf("\r Found %v Failed Files \n", count)

	return count
}

// func RetryDownload(retryCount int, timout float64) {
// 	tasks := GatherFailedURLs()
// 	if len(tasks) <= 0 {
// 		log.Println("Nothing To Retry")
// 		return
// 	}

// 	fmt.Println("Retrying Download...")
// 	for task := range retryChan {
// 		DownloadWorker(task)
// 	}
// }
