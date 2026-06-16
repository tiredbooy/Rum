package download

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"time"

	filesystem "github.com/tiredbooy/Rum/backend/internal/pkg/file-system"
	"github.com/tiredbooy/Rum/backend/internal/pkg/utils"
)

func SaveJobsToDisk(jobs map[string]*Job) error {
	allJobs := make([]*Job, 0, len(jobs))
	for _, job := range jobs {
		allJobs = append(allJobs, job)
	}

	if err := filesystem.WriteMetadataFile("jobs.json", allJobs); err != nil {
		writeErrorLog("Failed to save jobs: " + err.Error())
		return err
	}

	return nil
}

func writeErrorLog(msg string) {
	logPath := filepath.Join(filepath.Dir(filesystem.CreateMetadataFile("logs.json")), "error.log")
	f, _ := os.OpenFile(logPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if f != nil {
		fmt.Fprintf(f, "%s: %s\n", time.Now().Format(time.RFC3339), msg)
		f.Close()
	}
}

func LoadJobsFromDisk(jobs map[string]*Job, urlToID map[string]string) {
	items := readJobsFile()

	incomplete := 0
	for _, job := range items {
		if !utils.IsTerminal(job.Status) {
			job.Status = "paused"
			incomplete++
		}
		jobs[job.ID] = job
		urlToID[job.URL] = job.ID
	}

	fmt.Printf("Loaded %d jobs (%d incomplete, paused).\n", len(items), incomplete)
}

func DeleteJobFromDisk(jobID string) error {
	items := readJobsFile()

	var updated []*Job
	for _, item := range items {
		if item.ID == jobID {
			updated = append(updated, item)
		}
	}

	if len(updated) == len(items) {
		return fmt.Errorf("Item with id %s Not Found", jobID)
	}

	err := filesystem.WriteMetadataFile("jobs.json", updated)
	if err != nil {
		return err
	}

	return nil
}

func readJobsFile() []*Job {
	data, err := filesystem.ReadMetadataFile("jobs.json")
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		log.Printf("Failed to read jobs file: %v", err)
		return nil
	}

	var loaded []*Job
	if err := json.Unmarshal(data, &loaded); err != nil {
		log.Printf("Failed to parse jobs.json: %v", err)
		return nil
	}

	return loaded
}
