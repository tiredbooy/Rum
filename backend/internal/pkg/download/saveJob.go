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

func toQueueJob(job *Job) *Job {
	return &Job{
		ID:         job.ID,
		URL:        job.URL,
		Status:     job.Status,
		OutputPath: job.OutputPath,
		Downloaded: job.Downloaded,
		TotalSize:  job.TotalSize,
	}
}

func toHistoryJob(job *Job) *Job {
	return &Job{
		ID:          job.ID,
		URL:         job.URL,
		Status:      job.Status,
		OutputPath:  job.OutputPath,
		Downloaded:  job.Downloaded,
		TotalSize:   job.TotalSize,
		CompletedAt: job.CompletedAt,
		CreatedAt:   job.CreatedAt,
	}
}

func SaveJobsToDisk(jobs map[string]*Job) error {
	var activeJobs []*Job
	var historyJobs []*Job

	for _, job := range jobs {
		if job.Status == "running" || job.Status == "paused" {
			activeJobs = append(activeJobs, toQueueJob(job))
		} else {
			historyJobs = append(historyJobs, toHistoryJob(job))
		}
	}

	if len(activeJobs) == 0 {
		queuePath := filesystem.CreateMetadataFile("queue.json")
		if err := os.Remove(queuePath); err != nil && !os.IsNotExist(err) {
			writeErrorLog("Failed to remove queue.json: " + err.Error())
		}
	} else {
		if err := filesystem.WriteMetadataFile("queue.json", activeJobs); err != nil {
			writeErrorLog("Failed to save queue: " + err.Error())
			return err
		}
	}

	if err := filesystem.WriteMetadataFile("history.json", historyJobs); err != nil {
		writeErrorLog("Failed to save history: " + err.Error())
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
