package download

import (
	"fmt"
	"log"
)

func collectResult(expected int) {
	log.Println("Result: ", <-resultChan)
	log.Println("COLLECTED ")
	successCount := 0
	failureCount := 0
	var failedURLs []string

	for result := range resultChan {
		if result.Success {
			successCount++
			log.Println("Success Count: ", successCount)
		} else {
			failureCount++
			log.Println("Failure Count: ", failureCount)
			if result.Error != nil {
				failedURLs = append(failedURLs, fmt.Sprintf("%s (%s)", result.URL, result.Error.Error()))
			} else {
				failedURLs = append(failedURLs, result.URL)
			}
		}
	}

	fmt.Printf("\nDownload complete. Success: %d, Failed: %d\n", successCount, failureCount)

	if failureCount > 0 {
		fmt.Println("Failed URLs: ")
		for _, url := range failedURLs {
			fmt.Printf("- %s\n", url)
		}
	}
}
