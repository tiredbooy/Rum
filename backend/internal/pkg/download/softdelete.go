package download

import (
	"context"
	"fmt"
	"os"
	"time"
)

// Soft delete: "delete" hides the job and pauses its transfer, then the actual
// removal runs when the undo window ends. The deadline is persisted
// (Job.PendingDeleteAt) so a restart finishes an expired delete or restores a
// live one, never leaving a hidden zombie. Two flavours:
//
//	deleteFile=false  "Remove from list": file, partial and sidecar stay on disk.
//	deleteFile=true   "Delete file": also removes them (a finished file too).

// defaultUndoWindow is a little longer than the 10s toast so a click on Undo at
// the last moment still lands.
const defaultUndoWindow = 12 * time.Second

func (j *Job) GetPendingDeleteAt() time.Time {
	j.Mu.RLock()
	defer j.Mu.RUnlock()
	return j.PendingDeleteAt
}
func (j *Job) SetPendingDeleteAt(t time.Time) {
	j.Mu.Lock()
	defer j.Mu.Unlock()
	j.PendingDeleteAt = t
}
func (j *Job) GetPendingDeleteFile() bool {
	j.Mu.RLock()
	defer j.Mu.RUnlock()
	return j.PendingDeleteFile
}

// IsPendingDelete reports whether the job is hidden awaiting its delete deadline.
func (j *Job) IsPendingDelete() bool { return !j.GetPendingDeleteAt().IsZero() }

func (j *Job) clearPendingDelete() {
	j.Mu.Lock()
	defer j.Mu.Unlock()
	j.PendingDeleteAt, j.PendingDeleteFile, j.PendingDeleteResume = time.Time{}, false, false
}

// SetUndoWindow overrides how long a delete can be undone (tests).
func (m *JobManager) SetUndoWindow(d time.Duration) {
	m.mu.Lock()
	m.undoWindow = d
	m.mu.Unlock()
}

func (m *JobManager) undoWindowLocked() time.Duration {
	if m.undoWindow > 0 {
		return m.undoWindow
	}
	return defaultUndoWindow
}

// SoftDeleteJob hides a job, pauses its transfer and schedules the removal.
// It returns the deadline after which RestoreJob no longer works.
func (m *JobManager) SoftDeleteJob(jobID string, deleteFile bool) (time.Time, error) {
	if jobID == "" {
		return time.Time{}, fmt.Errorf("job id is not valid")
	}
	m.mu.Lock()
	job, ok := m.jobs[jobID]
	if !ok || job.IsPendingDelete() {
		m.mu.Unlock()
		return time.Time{}, fmt.Errorf("job %s not found", jobID)
	}
	at := m.softDeleteLocked(job, deleteFile)
	m.mu.Unlock()
	m.saveToDisk()
	return at, nil
}

// SoftDeleteJobsByFilter is the bulk form ("all", "completed", "error"/"failed").
// It returns the ids hidden so the caller can undo them together.
func (m *JobManager) SoftDeleteJobsByFilter(filter string, deleteFile bool) ([]string, time.Time, error) {
	if filter == "" {
		filter = "all"
	}
	m.mu.Lock()
	var ids []string
	var at time.Time
	for _, job := range m.jobs {
		if job.IsPendingDelete() || !shouldDeleteForFilter(job, filter) {
			continue
		}
		at = m.softDeleteLocked(job, deleteFile)
		ids = append(ids, job.ID)
	}
	m.mu.Unlock()
	if len(ids) > 0 {
		m.saveToDisk()
	}
	return ids, at, nil
}

// softDeleteLocked marks the job and arms its timer. Caller holds m.mu.
func (m *JobManager) softDeleteLocked(job *Job, deleteFile bool) time.Time {
	status := job.GetStatus()
	resume := status == StatusRunning || status == StatusPending
	if cancel := job.GetCancelFunc(); cancel != nil && status == StatusRunning {
		cancel() // pause the transfer; the partial stays until the deadline
	}
	if status == StatusPending {
		job.SetStatus(StatusPaused) // the dispatcher skips non-queued jobs
	}
	window := m.undoWindowLocked()
	at := time.Now().Add(window)
	job.Mu.Lock()
	job.PendingDeleteAt, job.PendingDeleteFile, job.PendingDeleteResume = at, deleteFile, resume
	job.Mu.Unlock()
	if m.urls[job.URL] == job.ID {
		delete(m.urls, job.URL) // a re-add of the same link must not hit a hidden job
	}
	if m.deleteTimers == nil {
		m.deleteTimers = make(map[string]*time.Timer)
	}
	id := job.ID
	m.deleteTimers[id] = time.AfterFunc(window, func() { m.finalizeDelete(id) })
	return at
}

// RestoreJob cancels a pending delete. It fails once the job is gone.
func (m *JobManager) RestoreJob(jobID string) error {
	m.mu.Lock()
	job, ok := m.jobs[jobID]
	if !ok || !job.IsPendingDelete() {
		m.mu.Unlock()
		return fmt.Errorf("job %s not found", jobID)
	}
	if other, taken := m.urls[job.URL]; taken && other != jobID {
		m.mu.Unlock()
		return fmt.Errorf("job %s already exists for this url", jobID)
	}
	if t := m.deleteTimers[jobID]; t != nil {
		t.Stop()
		delete(m.deleteTimers, jobID)
	}
	job.Mu.RLock()
	resume := job.PendingDeleteResume
	job.Mu.RUnlock()
	job.clearPendingDelete()
	m.urls[job.URL] = jobID
	m.mu.Unlock()
	// An Undo must not start a night job outside its window: park it for the
	// night controller instead (it restarts night-paused jobs when it opens).
	if resume && m.NightHolds(job, time.Now()) {
		job.SetNightPaused(true)
		resume = false
	}
	m.saveToDisk()
	if resume {
		_ = m.StartJob(context.Background(), jobID)
	}
	return nil
}

// RestoreJobs undoes a bulk delete; it returns how many were restored.
func (m *JobManager) RestoreJobs(ids []string) int {
	n := 0
	for _, id := range ids {
		if m.RestoreJob(id) == nil {
			n++
		}
	}
	return n
}

// finalizeDelete runs at the deadline: drop the job and, if asked, its data.
func (m *JobManager) finalizeDelete(jobID string) {
	m.mu.Lock()
	job, ok := m.jobs[jobID]
	if !ok || !job.IsPendingDelete() {
		m.mu.Unlock()
		return
	}
	m.removeExpiredLocked(job)
	m.mu.Unlock()
	m.saveToDisk()
}

// removeExpiredLocked drops a pending-delete job and removes its data per its
// flag. Caller holds m.mu. Disk I/O is small (a couple of unlinks).
func (m *JobManager) removeExpiredLocked(job *Job) {
	if cancel := job.GetCancelFunc(); cancel != nil {
		cancel()
	}
	if t := m.deleteTimers[job.ID]; t != nil {
		t.Stop()
		delete(m.deleteTimers, job.ID)
	}
	delete(m.jobs, job.ID)
	if m.urls[job.URL] == job.ID {
		delete(m.urls, job.URL)
	}
	if !job.GetPendingDeleteFile() {
		return
	}
	path := job.GetOutputPath()
	if path == "" || m.pathSharedLocked(job, path) {
		return // a re-added link now owns this file/partial
	}
	if job.GetStatus() == StatusCompleted {
		removePartsMeta(path)
		_ = os.Remove(path) // explicit "Delete file" is the only path that removes a finished file
		return
	}
	RemovePartialData(path)
}

// pathSharedLocked reports whether another job uses the same output path, e.g.
// the same link re-added during the undo window. Caller holds m.mu.
func (m *JobManager) pathSharedLocked(job *Job, path string) bool {
	for id, other := range m.jobs {
		if id != job.ID && other.GetOutputPath() == path {
			return true
		}
	}
	return false
}

// resolvePendingDeletesLocked runs at startup. A delete whose deadline passed
// while the app was closed is executed; one still inside its window is restored
// (the timer died with the process). Caller holds m.mu.
func (m *JobManager) resolvePendingDeletesLocked() {
	changed := false
	now := time.Now()
	for _, job := range m.snapshotJobsLocked() {
		at := job.GetPendingDeleteAt()
		if at.IsZero() {
			continue
		}
		changed = true
		if !at.After(now) {
			job.SetCancelFunc(nil)
			m.removeExpiredLocked(job)
			continue
		}
		job.Mu.Lock()
		if job.PendingDeleteResume {
			job.Interrupted = true // resume-on-launch picks it up
		}
		job.Mu.Unlock()
		job.clearPendingDelete()
		m.urls[job.URL] = job.ID
	}
	if changed {
		m.saveToDiskLocked()
	}
}

func (m *JobManager) snapshotJobsLocked() []*Job { return m.saveJobsSnapshotLocked() }

// stopDeleteTimers cancels pending deadline timers on shutdown; the persisted
// PendingDeleteAt lets the next launch resolve them.
func (m *JobManager) stopDeleteTimers() {
	m.mu.Lock()
	for id, t := range m.deleteTimers {
		t.Stop()
		delete(m.deleteTimers, id)
	}
	m.mu.Unlock()
}
