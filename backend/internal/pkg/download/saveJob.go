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
	return PersistJobs(allJobs)
}

// PersistJobs atomically writes a pre-built jobs slice to jobs.json. Callers that
// hold the manager lock pass an already-snapshotted slice, so the live jobs map
// is never ranged outside the lock (which would be a fatal "concurrent map
// iteration and map write"). Each *Job marshals under its own lock.
func PersistJobs(jobs []*Job) error {
	backupCurrentJobs(filesystem.CreateMetadataFile("jobs.json"))
	// 0600: jobs may carry a browser cookie kept for resume.
	if err := writeJobsFile(jobs); err != nil {
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

// LoadJobsFromDisk restores the previous session's jobs. Finished jobs and
// errored jobs keep their state (an error keeps its message, and stays eligible
// for auto-resume on reconnect). A job that was running or queued is paused and
// flagged Interrupted so ResumeInterruptedJobs can tell it apart from a job the
// user paused. A pending job waiting on a future scheduled start is left pending
// for the schedule controller.
func LoadJobsFromDisk(jobs map[string]*Job, urlToID map[string]string) {
	items := readJobsFile()

	interrupted := 0
	for _, job := range items {
		switch {
		case utils.IsTerminal(job.Status), job.Status == StatusError:
			// keep as-is
		case job.Status == StatusPending && job.StartAt.After(time.Now()):
			// keep pending until the schedule controller starts it
		case job.Status == StatusPending && job.NightQueue:
			// keep pending until the night window opens
		case job.Status == StatusRunning || job.Status == StatusPending:
			job.Status = StatusPaused
			job.Interrupted = true
			interrupted++
		default:
			job.Status = StatusPaused
		}
		jobs[job.ID] = job
		urlToID[job.URL] = job.ID
	}

	fmt.Printf("Loaded %d jobs (%d interrupted, paused).\n", len(items), interrupted)
}

// readJobsFile loads jobs.json with backup recovery (see jobs_backup.go).
func readJobsFile() []*Job {
	return loadJobsWithRecovery(filesystem.CreateMetadataFile("jobs.json"))
}

// decodeJobs parses a jobs.json payload. ok=false means nothing usable could be
// read (not a JSON array); skipped counts records dropped by the lenient path.
func decodeJobs(data []byte) (jobs []*Job, skipped int, ok bool) {
	// Fast path: decode the whole array at once.
	var loaded []*Job
	if err := json.Unmarshal(data, &loaded); err == nil {
		return loaded, 0, true
	} else {
		// One malformed record must never wipe the user's entire history. Fall back
		// to decoding record-by-record and keep every job that still parses.
		log.Printf("jobs.json array decode failed (%v); recovering record-by-record", err)
	}

	var raws []json.RawMessage
	if err := json.Unmarshal(data, &raws); err != nil {
		log.Printf("Failed to parse jobs.json: %v", err)
		return nil, 0, false
	}
	recovered := make([]*Job, 0, len(raws))
	for i, raw := range raws {
		var j Job
		if err := json.Unmarshal(raw, &j); err != nil {
			log.Printf("skipping unreadable job #%d in jobs.json: %v", i, err)
			skipped++
			continue
		}
		recovered = append(recovered, &j)
	}
	return recovered, skipped, true
}

func writeJobsFile(jobs []*Job) error {
	data, err := json.MarshalIndent(jobs, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal jobs.json: %w", err)
	}
	return filesystem.AtomicWriteFile(filesystem.CreateMetadataFile("jobs.json"), data, 0o600)
}
