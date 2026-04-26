package download

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"time"
)

func GetJobsFilePath() string {
	configDir, err := os.UserConfigDir()
	if err != nil {
		configDir = ".rum"
	}
	dir := filepath.Join(configDir, "rum")
	os.MkdirAll(dir, 0755)
	return filepath.Join(dir, "queue.json")
}

func SaveJobsToDisk() error {
	mu.Lock()
	defer mu.Unlock()

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

	path := GetJobsFilePath()
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
	logPath := filepath.Join(filepath.Dir(GetJobsFilePath()), "error.log")
	f, _ := os.OpenFile(logPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if f != nil {
		fmt.Fprintf(f, "%s: %s\n", time.Now().Format(time.RFC3339), msg)
		f.Close()
	}
}

func LoadJobsFromDisk() {
	path := GetJobsFilePath()
	data, err := os.ReadFile(path)
	if err != nil {
		return
	}

	var loadedJobs []*Job
	if err := json.Unmarshal(data, &loadedJobs); err != nil {
		log.Printf("Failed to parse saved jobs: %v", err)
		return
	}

	fmt.Printf("Found %d incomplete downloads. Resume later from the TUI.\n", len(loadedJobs))
	for _, j := range loadedJobs {
		j.Status = "paused"
		mu.Lock()
		jobs[j.ID] = j
		mu.Unlock()
	}
}
