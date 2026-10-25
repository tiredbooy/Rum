package download

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	filesystem "github.com/tiredbooy/Rum/backend/internal/pkg/file-system"
)

func jobsPath() string { return filesystem.CreateMetadataFile("jobs.json") }

func globCorrupt(t *testing.T, suffix string) []string {
	t.Helper()
	m, err := filepath.Glob(jobsPath() + suffix + ".corrupt-*")
	if err != nil {
		t.Fatal(err)
	}
	return m
}

func TestPersistJobsKeepsPreviousFileAsBak(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	a := &Job{ID: "a", URL: "http://x/a", Status: StatusPaused}
	b := &Job{ID: "b", URL: "http://x/b", Status: StatusPaused}

	if err := PersistJobs([]*Job{a}); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(jobsPath() + ".bak"); err == nil {
		t.Fatal("first save has nothing to back up")
	}
	if err := PersistJobs([]*Job{a, b}); err != nil {
		t.Fatal(err)
	}
	bak, err := os.ReadFile(jobsPath() + ".bak")
	if err != nil {
		t.Fatalf("expected jobs.json.bak: %v", err)
	}
	if strings.Contains(string(bak), `"b"`) || !strings.Contains(string(bak), `"a"`) {
		t.Fatalf("bak should hold the previous list only: %s", bak)
	}
}

func TestPersistJobsNeverBacksUpCorruptFile(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	a := &Job{ID: "a", URL: "http://x/a", Status: StatusPaused}
	_ = PersistJobs([]*Job{a})
	_ = PersistJobs([]*Job{a}) // bak = good
	if err := os.WriteFile(jobsPath(), []byte("{garbage"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := PersistJobs([]*Job{a}); err != nil {
		t.Fatal(err)
	}
	bak, _ := os.ReadFile(jobsPath() + ".bak")
	if strings.Contains(string(bak), "garbage") || !strings.Contains(string(bak), `"a"`) {
		t.Fatalf("corrupt file leaked into bak: %s", bak)
	}
}

func TestLoadCorruptJobsRestoresBackupAndKeepsCorrupt(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	TakeJobsRestored() // reset
	a := &Job{ID: "a", URL: "http://x/a", Status: StatusPaused}
	_ = PersistJobs([]*Job{a})
	_ = PersistJobs([]*Job{a})
	if err := os.WriteFile(jobsPath(), []byte("{not json"), 0o644); err != nil {
		t.Fatal(err)
	}

	jobs, urls := map[string]*Job{}, map[string]string{}
	LoadJobsFromDisk(jobs, urls)

	if jobs["a"] == nil {
		t.Fatalf("job should be restored from .bak, got %v", jobs)
	}
	c := globCorrupt(t, "")
	if len(c) != 1 {
		t.Fatalf("want 1 corrupt copy, got %v", c)
	}
	if b, _ := os.ReadFile(c[0]); string(b) != "{not json" {
		t.Fatalf("corrupt copy must keep original bytes, got %q", b)
	}
	if !TakeJobsRestored() {
		t.Fatal("restore notice expected")
	}
	if TakeJobsRestored() {
		t.Fatal("notice must be handed out once")
	}
	if b, _ := os.ReadFile(jobsPath()); !strings.Contains(string(b), `"a"`) {
		t.Fatalf("jobs.json should be valid again: %s", b)
	}
}

func TestLoadBothCorruptStartsEmptyKeepsFiles(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	TakeJobsRestored()
	_ = os.MkdirAll(filepath.Dir(jobsPath()), 0o755)
	_ = os.WriteFile(jobsPath(), []byte("bad-main"), 0o644)
	_ = os.WriteFile(jobsPath()+".bak", []byte("bad-bak"), 0o644)

	jobs, urls := map[string]*Job{}, map[string]string{}
	LoadJobsFromDisk(jobs, urls)

	if len(jobs) != 0 {
		t.Fatalf("expected empty, got %v", jobs)
	}
	if TakeJobsRestored() {
		t.Fatal("no restore notice when nothing was restored")
	}
	if len(globCorrupt(t, "")) != 1 || len(globCorrupt(t, ".bak")) != 1 {
		t.Fatal("both corrupt files must be kept")
	}
	// A later save must not destroy the kept copies.
	_ = PersistJobs(nil)
	_ = PersistJobs(nil)
	if len(globCorrupt(t, "")) != 1 || len(globCorrupt(t, ".bak")) != 1 {
		t.Fatal("corrupt copies must survive later saves")
	}
}

func TestCorruptCopiesAreNeverOverwritten(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	_ = os.MkdirAll(filepath.Dir(jobsPath()), 0o755)
	for i := 0; i < 3; i++ {
		_ = os.WriteFile(jobsPath(), []byte("bad"+string(rune('0'+i))), 0o644)
		LoadJobsFromDisk(map[string]*Job{}, map[string]string{})
	}
	if n := len(globCorrupt(t, "")); n != 3 {
		t.Fatalf("want 3 distinct corrupt copies, got %d", n)
	}
}

func TestLoadMissingJobsFileIsFirstRun(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	TakeJobsRestored()
	LoadJobsFromDisk(map[string]*Job{}, map[string]string{})
	if TakeJobsRestored() || len(globCorrupt(t, "")) != 0 {
		t.Fatal("a fresh install must not look like a recovery")
	}
}
