package download

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"sync/atomic"
	"time"

	filesystem "github.com/tiredbooy/Rum/backend/internal/pkg/file-system"
)

// jobsRestored is set when jobs.json was unreadable and the list was restored
// from jobs.json.bak. The window is not mounted yet at load time, so the desktop
// shell asks once via TakeJobsRestored (same pattern as the resume count).
var jobsRestored atomic.Bool

// TakeJobsRestored reports, once, whether the list was restored from backup.
func TakeJobsRestored() bool { return jobsRestored.Swap(false) }

// backupCurrentJobs keeps the previous good jobs.json as jobs.json.bak just
// before it is replaced. A file that is not a valid job list is never copied
// over the backup, so a corrupt save cannot poison the last good copy.
func backupCurrentJobs(path string) {
	data, err := os.ReadFile(path)
	if err != nil {
		return
	}
	var probe []json.RawMessage
	if json.Unmarshal(data, &probe) != nil {
		return
	}
	if err := filesystem.AtomicWriteFile(path+".bak", data, 0o600) // may hold a browser cookie; err != nil {
		log.Printf("jobs backup failed: %v", err)
	}
}

// keepCorrupt moves an unusable file aside as <path>.corrupt-<timestamp>. The
// name is never reused, so an earlier corrupt copy is never overwritten.
func keepCorrupt(path string) {
	if _, err := os.Stat(path); err != nil {
		return
	}
	base := fmt.Sprintf("%s.corrupt-%s", path, time.Now().Format("20060102-150405"))
	dst := base
	for n := 1; ; n++ {
		if _, err := os.Lstat(dst); os.IsNotExist(err) {
			break
		}
		dst = fmt.Sprintf("%s-%d", base, n)
	}
	if err := os.Rename(path, dst); err != nil {
		log.Printf("could not set aside corrupt %s: %v", path, err)
		return
	}
	log.Printf("kept corrupt file as %s", dst)
}

// copyCorrupt keeps a copy of a partly readable file whose bad records were
// dropped, so nothing the user could still recover by hand is lost.
func copyCorrupt(path string, data []byte) {
	dst := fmt.Sprintf("%s.corrupt-%s", path, time.Now().Format("20060102-150405"))
	for n := 1; ; n++ {
		if _, err := os.Lstat(dst); os.IsNotExist(err) {
			break
		}
		dst = fmt.Sprintf("%s.corrupt-%s-%d", path, time.Now().Format("20060102-150405"), n)
	}
	if err := os.WriteFile(dst, data, 0o644); err != nil {
		log.Printf("could not keep copy of %s: %v", path, err)
	}
}

// loadJobsWithRecovery reads the job list at path. If the file is unreadable or
// corrupt it is kept aside and the .bak is loaded instead; if that fails too the
// list starts empty with both corrupt files preserved.
func loadJobsWithRecovery(path string) []*Job {
	data, err := os.ReadFile(path)
	switch {
	case err == nil:
		jobs, skipped, ok := decodeJobs(data)
		if ok {
			if skipped > 0 {
				copyCorrupt(path, data)
			}
			return jobs
		}
	case os.IsNotExist(err):
		if _, berr := os.Stat(path + ".bak"); berr != nil {
			return nil // first run
		}
	default:
		log.Printf("Failed to read jobs file: %v", err)
	}

	keepCorrupt(path)

	bak, err := os.ReadFile(path + ".bak")
	if err != nil {
		return nil
	}
	jobs, _, ok := decodeJobs(bak)
	if !ok {
		keepCorrupt(path + ".bak")
		return nil
	}
	if err := filesystem.AtomicWriteFile(path, bak, 0o644); err != nil {
		log.Printf("could not rewrite jobs.json from backup: %v", err)
	}
	jobsRestored.Store(true)
	log.Printf("jobs.json was unreadable; restored %d jobs from backup", len(jobs))
	return jobs
}
