package download

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"time"

	filesystem "github.com/tiredbooy/Rum/backend/internal/pkg/file-system"
)



func SaveJobsToDisk(jobs map[string]*Job) error {

	var activeJobs []*Job
	for _, job := range jobs {
		if job.Status == "running" || job.Status == "paused" {
			copyJob := &Job{
				ID:         job.ID,
				URL:        job.URL,
				Status:     job.Status,
				OutputPath: job.OutputPath,
				Downloaded: job.Downloaded,
				TotalSize:  job.TotalSize,
			}
			activeJobs = append(activeJobs, copyJob)
		}
	}

	path := filesystem.CreateMetadataFile("queue.json")
	if len(activeJobs) == 0 {
		if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
			writeErrorLog("Failed to remove job file: " + err.Error())
		}
		return nil
	}

	data, err := json.MarshalIndent(activeJobs, "", "  ")
	if err != nil {
		writeErrorLog("JSON marshal error: " + err.Error())
		return err
	}

	if err := os.WriteFile(path, data, 0644); err != nil {
		writeErrorLog("WriteFile error: " + err.Error())
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

	data, err := filesystem.ReadMetadataFile("queue.json")
	if err != nil {
		return
	}

	var loaded []*Job
	if err := json.Unmarshal(data, &loaded); err != nil {
		log.Printf("Failed to parse saved jobs: %v", err)
		return
	}
	fmt.Printf("Found %d incomplete downloads. Resume later from the TUI.\n", len(jobs))
	for _, job := range loaded {
		job.Status = "paused"
		jobs[job.ID] = job
		urlToID[job.URL] = job.ID
	}
	fmt.Printf("Loaded %d incomplete downloads.\n", len(loaded))
}
