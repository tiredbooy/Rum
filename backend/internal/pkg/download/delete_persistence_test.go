package download

import (
	"path/filepath"
	"testing"
)

// TestDeleteJobKeepsOtherJobsOnDisk is a regression test for the inverted-filter
// data-loss bug: deleting one job must leave every OTHER job persisted in
// jobs.json. The old DeleteJobFromDisk re-read jobs.json and kept ONLY the
// deleted job, wiping the user's entire history on every single delete.
func TestDeleteJobKeepsOtherJobsOnDisk(t *testing.T) {
	m, outDir := newManagerForTest(t, 1)
	defer m.Shutdown()

	ids := []string{"job-a", "job-b", "job-c"}
	for _, id := range ids {
		j := &Job{
			ID:         id,
			URL:        "http://example.com/" + id,
			Status:     StatusCompleted,
			OutputPath: filepath.Join(outDir, id+".bin"),
		}
		m.mu.Lock()
		m.jobs[id] = j
		m.urls[j.URL] = id
		m.mu.Unlock()
	}
	m.saveToDisk()

	if err := m.DeleteJob("job-b"); err != nil {
		t.Fatalf("DeleteJob returned error: %v", err)
	}

	got := readJobsFile()
	if len(got) != 2 {
		t.Fatalf("expected 2 jobs on disk after deleting 1 of 3, got %d", len(got))
	}
	survived := map[string]bool{}
	for _, j := range got {
		survived[j.ID] = true
	}
	if !survived["job-a"] || !survived["job-c"] || survived["job-b"] {
		t.Fatalf("wrong survivors after delete (a,c should remain, b gone): %+v", survived)
	}
}
