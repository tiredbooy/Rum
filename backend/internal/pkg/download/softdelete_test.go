package download

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

// addFileJob registers a job whose output file really exists on disk.
func addFileJob(t *testing.T, m *JobManager, dir, id, status string) *Job {
	t.Helper()
	p := filepath.Join(dir, id+".bin")
	if err := os.WriteFile(p, []byte("data"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(partsFilePath(p), []byte("{}"), 0o644); err != nil {
		t.Fatal(err)
	}
	j := &Job{ID: id, URL: "http://example.com/" + id, Status: status, OutputPath: p}
	m.mu.Lock()
	m.jobs[id] = j
	m.urls[j.URL] = id
	m.mu.Unlock()
	return j
}

func exists(p string) bool { _, err := os.Stat(p); return err == nil }

func waitGone(t *testing.T, m *JobManager, id string) {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		m.mu.RLock()
		_, ok := m.jobs[id]
		m.mu.RUnlock()
		if !ok {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("job %s still present after undo window", id)
}

func TestSoftDeleteHidesThenDeletesFileOnExpiry(t *testing.T) {
	m, dir := newManagerForTest(t, 1)
	defer m.Shutdown()
	m.SetUndoWindow(60 * time.Millisecond)
	j := addFileJob(t, m, dir, "a", StatusCompleted)

	at, err := m.SoftDeleteJob("a", true)
	if err != nil {
		t.Fatal(err)
	}
	if !at.After(time.Now().Add(-time.Second)) {
		t.Fatalf("deadline not set: %v", at)
	}
	if _, ok := m.GetJob("a"); ok {
		t.Fatal("soft-deleted job still visible via GetJob")
	}
	for _, x := range m.GetAllJobs() {
		if x.ID == "a" {
			t.Fatal("soft-deleted job still listed")
		}
	}
	if !exists(j.OutputPath) {
		t.Fatal("file removed before undo window expired")
	}
	waitGone(t, m, "a")
	if exists(j.OutputPath) || exists(partsFilePath(j.OutputPath)) {
		t.Fatal("delete-file expiry must remove finished file and sidecar")
	}
}

func TestSoftDeleteRemoveFromListKeepsData(t *testing.T) {
	m, dir := newManagerForTest(t, 1)
	defer m.Shutdown()
	m.SetUndoWindow(40 * time.Millisecond)
	done := addFileJob(t, m, dir, "done", StatusCompleted)
	part := addFileJob(t, m, dir, "part", StatusPaused)

	for _, id := range []string{"done", "part"} {
		if _, err := m.SoftDeleteJob(id, false); err != nil {
			t.Fatal(err)
		}
	}
	waitGone(t, m, "done")
	waitGone(t, m, "part")
	if !exists(done.OutputPath) || !exists(part.OutputPath) || !exists(partsFilePath(part.OutputPath)) {
		t.Fatal("remove-from-list must keep file, partial and sidecar on disk")
	}
}

func TestSoftDeleteIncompleteDeleteFileRemovesPartial(t *testing.T) {
	m, dir := newManagerForTest(t, 1)
	defer m.Shutdown()
	m.SetUndoWindow(40 * time.Millisecond)
	part := addFileJob(t, m, dir, "part", StatusPaused)
	if _, err := m.SoftDeleteJob("part", true); err != nil {
		t.Fatal(err)
	}
	waitGone(t, m, "part")
	if exists(part.OutputPath) || exists(partsFilePath(part.OutputPath)) {
		t.Fatal("partial and sidecar should be removed")
	}
}

func TestRestoreWithinWindowCancelsDelete(t *testing.T) {
	m, dir := newManagerForTest(t, 1)
	defer m.Shutdown()
	m.SetUndoWindow(80 * time.Millisecond)
	j := addFileJob(t, m, dir, "a", StatusCompleted)

	if _, err := m.SoftDeleteJob("a", true); err != nil {
		t.Fatal(err)
	}
	if err := m.RestoreJob("a"); err != nil {
		t.Fatal(err)
	}
	if _, ok := m.GetJob("a"); !ok {
		t.Fatal("restored job not visible")
	}
	if id, ok := m.GetJobIDByURL(j.URL); !ok || id != "a" {
		t.Fatal("URL index not restored")
	}
	time.Sleep(200 * time.Millisecond)
	if _, ok := m.GetJob("a"); !ok || !exists(j.OutputPath) {
		t.Fatal("restored job must survive the old deadline")
	}
	if !j.GetPendingDeleteAt().IsZero() {
		t.Fatal("pending_delete_at not cleared")
	}
}

func TestRestoreAfterExpiryFails(t *testing.T) {
	m, dir := newManagerForTest(t, 1)
	defer m.Shutdown()
	m.SetUndoWindow(20 * time.Millisecond)
	addFileJob(t, m, dir, "a", StatusCompleted)
	if _, err := m.SoftDeleteJob("a", false); err != nil {
		t.Fatal(err)
	}
	waitGone(t, m, "a")
	if err := m.RestoreJob("a"); err == nil {
		t.Fatal("restore after expiry must fail")
	}
}

func TestSoftDeleteCancelsRunningTransferAndRestoreResumesIt(t *testing.T) {
	m, dir := newManagerForTest(t, 1)
	defer m.Shutdown()
	m.SetUndoWindow(time.Minute)
	j := addFileJob(t, m, dir, "r", StatusRunning)
	cancelled := false
	j.SetCancelFunc(func() { cancelled = true })

	if _, err := m.SoftDeleteJob("r", true); err != nil {
		t.Fatal(err)
	}
	if !cancelled {
		t.Fatal("running transfer must be paused on soft delete")
	}
	if err := m.RestoreJob("r"); err != nil {
		t.Fatal(err)
	}
	if st := j.GetStatus(); st != StatusPending && st != StatusRunning {
		t.Fatalf("restored active job should be re-queued, status=%s", st)
	}
}

func TestSoftDeleteUnknownAndRepeat(t *testing.T) {
	m, dir := newManagerForTest(t, 1)
	defer m.Shutdown()
	m.SetUndoWindow(time.Minute)
	if _, err := m.SoftDeleteJob("nope", false); err == nil {
		t.Fatal("unknown id must error")
	}
	addFileJob(t, m, dir, "a", StatusPaused)
	if _, err := m.SoftDeleteJob("a", false); err != nil {
		t.Fatal(err)
	}
	if _, err := m.SoftDeleteJob("a", false); err == nil {
		t.Fatal("deleting a hidden job again must error")
	}
}

func TestSoftDeleteByFilterAndBulkRestore(t *testing.T) {
	m, dir := newManagerForTest(t, 1)
	defer m.Shutdown()
	m.SetUndoWindow(time.Minute)
	addFileJob(t, m, dir, "c1", StatusCompleted)
	addFileJob(t, m, dir, "c2", StatusCompleted)
	addFileJob(t, m, dir, "p", StatusPaused)

	ids, _, err := m.SoftDeleteJobsByFilter("completed", false)
	if err != nil || len(ids) != 2 {
		t.Fatalf("ids=%v err=%v", ids, err)
	}
	if got := len(m.GetAllJobs()); got != 1 {
		t.Fatalf("want 1 visible job, got %d", got)
	}
	if n := m.RestoreJobs(ids); n != 2 {
		t.Fatalf("restored %d, want 2", n)
	}
	if got := len(m.GetAllJobs()); got != 3 {
		t.Fatalf("want 3 visible jobs, got %d", got)
	}
}

func TestPendingDeleteSurvivesRestart(t *testing.T) {
	m, dir := newManagerForTest(t, 1)
	defer m.Shutdown()
	m.SetUndoWindow(time.Minute)
	expired := addFileJob(t, m, dir, "expired", StatusCompleted)
	live := addFileJob(t, m, dir, "live", StatusCompleted)
	keep := addFileJob(t, m, dir, "keep", StatusCompleted)

	if _, err := m.SoftDeleteJob("expired", true); err != nil {
		t.Fatal(err)
	}
	if _, err := m.SoftDeleteJob("live", true); err != nil {
		t.Fatal(err)
	}
	expired.SetPendingDeleteAt(time.Now().Add(-time.Second)) // deadline passed while app was closed
	m.saveToDisk()

	// Simulate relaunch from jobs.json.
	m.mu.Lock()
	m.jobs = map[string]*Job{}
	m.urls = map[string]string{}
	m.loadFromDisk()
	m.mu.Unlock()

	if _, ok := m.GetJob("expired"); ok || exists(expired.OutputPath) {
		t.Fatal("expired pending delete must be executed on restart")
	}
	got, ok := m.GetJob("live")
	if !ok || !exists(live.OutputPath) {
		t.Fatal("pending delete within window must be restored on restart")
	}
	if !got.GetPendingDeleteAt().IsZero() {
		t.Fatal("restored job still marked pending delete")
	}
	if _, ok := m.GetJob("keep"); !ok || !exists(keep.OutputPath) {
		t.Fatal("unrelated job lost")
	}
	// The cleanup must be persisted too.
	for _, pj := range readJobsFile() {
		if pj.ID == "expired" || !pj.GetPendingDeleteAt().IsZero() {
			t.Fatalf("jobs.json still carries %s / pending mark", pj.ID)
		}
	}
}

func TestPendingDeleteAtPersisted(t *testing.T) {
	m, dir := newManagerForTest(t, 1)
	defer m.Shutdown()
	m.SetUndoWindow(time.Minute)
	addFileJob(t, m, dir, "a", StatusCompleted)
	if _, err := m.SoftDeleteJob("a", true); err != nil {
		t.Fatal(err)
	}
	found := false
	for _, pj := range readJobsFile() {
		if pj.ID == "a" {
			found = true
			if pj.GetPendingDeleteAt().IsZero() || !pj.GetPendingDeleteFile() {
				t.Fatal("pending delete state not persisted")
			}
		}
	}
	if !found {
		t.Fatal("soft-deleted job must stay in jobs.json until finalized")
	}
}
