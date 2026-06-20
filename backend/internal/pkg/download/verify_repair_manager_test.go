package download

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
)

// seedCompletedJob writes a fully-downloaded file on disk and registers a
// matching completed job on the manager, returning the job id and the file path.
// totalSize is set to the file size so VerifyFile's structural/size check passes.
func seedCompletedJob(t *testing.T, m *JobManager, outDir string, content []byte) (string, string) {
	t.Helper()
	path := filepath.Join(outDir, "verify-me.bin")
	if err := os.WriteFile(path, content, 0644); err != nil {
		t.Fatalf("write completed file: %v", err)
	}
	job := &Job{
		ID:           "verify-job-1",
		URL:          "http://example.test/verify-me.bin",
		FileName:     "verify-me.bin",
		OutputPath:   path,
		TotalSize:    int64(len(content)),
		Downloaded:   int64(len(content)),
		SupportRange: true,
		Status:       StatusCompleted,
	}
	m.mu.Lock()
	m.jobs[job.ID] = job
	m.urls[job.URL] = job.ID
	m.mu.Unlock()
	return job.ID, path
}

// TestVerifyJobOnGoodCompletedFile: a completed, correctly-sized file with no
// sidecar verifies OK via the cheap structural path and never mutates the file.
func TestVerifyJobOnGoodCompletedFile(t *testing.T) {
	m, outDir := newManagerForTest(t, 1)
	defer m.Shutdown()

	content := []byte("hello, this is a complete and correct download payload")
	id, path := seedCompletedJob(t, m, outDir, content)

	res, err := m.VerifyJob(context.Background(), id, false)
	if err != nil {
		t.Fatalf("VerifyJob: %v", err)
	}
	if !res.Ok {
		t.Fatalf("VerifyJob result not ok: %+v", res)
	}
	if len(res.BadSegments) != 0 {
		t.Fatalf("expected no bad segments, got %v", res.BadSegments)
	}

	// Verify must not mutate the file.
	got, _ := os.ReadFile(path)
	if string(got) != string(content) {
		t.Fatal("VerifyJob must not modify the on-disk file")
	}
}

// TestVerifyJobNotFound: an unknown id surfaces ErrNotFound so the handler can
// map it to 404.
func TestVerifyJobNotFound(t *testing.T) {
	m, _ := newManagerForTest(t, 1)
	defer m.Shutdown()

	_, err := m.VerifyJob(context.Background(), "does-not-exist", false)
	if !errors.Is(err, ErrNotFound) {
		t.Fatalf("VerifyJob(unknown) error = %v, want ErrNotFound", err)
	}
}

// TestRepairJobIdempotentOnGoodFile: repairing an already-good completed job is a
// safe no-op that returns ok without re-downloading (segments==nil triggers a
// verify-first that finds nothing bad).
func TestRepairJobIdempotentOnGoodFile(t *testing.T) {
	m, outDir := newManagerForTest(t, 1)
	defer m.Shutdown()

	content := []byte("a wholly intact file needs no repair")
	id, path := seedCompletedJob(t, m, outDir, content)

	res, err := m.RepairJob(context.Background(), id, nil)
	if err != nil {
		t.Fatalf("RepairJob: %v", err)
	}
	if !res.Ok {
		t.Fatalf("RepairJob on a good file should be ok: %+v", res)
	}
	got, _ := os.ReadFile(path)
	if string(got) != string(content) {
		t.Fatal("RepairJob must not corrupt an already-good file")
	}
}

// TestRepairJobNotFound: an unknown id surfaces ErrNotFound.
func TestRepairJobNotFound(t *testing.T) {
	m, _ := newManagerForTest(t, 1)
	defer m.Shutdown()

	_, err := m.RepairJob(context.Background(), "nope", nil)
	if !errors.Is(err, ErrNotFound) {
		t.Fatalf("RepairJob(unknown) error = %v, want ErrNotFound", err)
	}
}

// TestDeleteJobsByFilterRemovesPartials: an explicit delete of a non-completed
// (errored) job removes its on-disk partial + sidecar, regardless of
// KeepPartialOnFailure, while a completed job's finished file is preserved.
func TestDeleteJobsByFilterRemovesPartials(t *testing.T) {
	m, outDir := newManagerForTest(t, 1)
	defer m.Shutdown()
	// KeepPartialOnFailure on: an EXPLICIT delete must still remove the partial.
	m.opt.KeepPartialOnFailure = true

	// Errored job with a partial file + sidecar on disk.
	partial := filepath.Join(outDir, "broken.bin")
	if err := os.WriteFile(partial, []byte("half a fil"), 0644); err != nil {
		t.Fatalf("write partial: %v", err)
	}
	sidecar := partsFilePath(partial)
	if err := os.WriteFile(sidecar, []byte("{}"), 0644); err != nil {
		t.Fatalf("write sidecar: %v", err)
	}
	errored := &Job{ID: "err-1", URL: "http://example.test/broken.bin", FileName: "broken.bin",
		OutputPath: partial, Status: StatusError}

	// Completed job whose finished file must be preserved on a status!=error delete.
	done := filepath.Join(outDir, "done.bin")
	if err := os.WriteFile(done, []byte("complete"), 0644); err != nil {
		t.Fatalf("write done: %v", err)
	}
	completed := &Job{ID: "done-1", URL: "http://example.test/done.bin", FileName: "done.bin",
		OutputPath: done, Status: StatusCompleted}

	m.mu.Lock()
	m.jobs[errored.ID] = errored
	m.jobs[completed.ID] = completed
	m.urls[errored.URL] = errored.ID
	m.urls[completed.URL] = completed.ID
	m.mu.Unlock()

	if err := m.DeleteJobsByFilter("error"); err != nil {
		t.Fatalf("DeleteJobsByFilter(error): %v", err)
	}

	if _, err := os.Stat(partial); !os.IsNotExist(err) {
		t.Errorf("partial file should be removed on explicit delete, stat err = %v", err)
	}
	if _, err := os.Stat(sidecar); !os.IsNotExist(err) {
		t.Errorf("sidecar should be removed on explicit delete, stat err = %v", err)
	}
	// The errored job is gone from the map; the completed one (not matched by the
	// "error" filter) and its file remain.
	m.mu.RLock()
	_, errStillThere := m.jobs[errored.ID]
	_, doneStillThere := m.jobs[completed.ID]
	m.mu.RUnlock()
	if errStillThere {
		t.Error("errored job should have been deleted")
	}
	if !doneStillThere {
		t.Error("completed job should NOT have been deleted by status=error filter")
	}
	if _, err := os.Stat(done); err != nil {
		t.Errorf("completed file must be preserved, stat err = %v", err)
	}
}
