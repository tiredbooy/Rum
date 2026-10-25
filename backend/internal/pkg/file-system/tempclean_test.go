package filesystem

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func tempFiles(t *testing.T, dir string) []string {
	t.Helper()
	m, _ := filepath.Glob(filepath.Join(dir, ".*.tmp-*"))
	return m
}

func TestAtomicWriteFileRemovesTempOnRenameFailure(t *testing.T) {
	dir := t.TempDir()
	orig := renameFile
	renameFile = func(string, string) error { return errors.New("injected") }
	defer func() { renameFile = orig }()

	for _, name := range []string{"jobs.json", "jobs.json.bak"} {
		if err := AtomicWriteFile(filepath.Join(dir, name), []byte("x"), 0o600); err == nil {
			t.Fatal("expected error")
		}
	}
	if left := tempFiles(t, dir); len(left) != 0 {
		t.Fatalf("temp files left behind: %v", left)
	}
}

func TestCleanStaleTempFiles(t *testing.T) {
	dir := t.TempDir()
	old := time.Now().Add(-time.Hour)
	mk := func(name string, mtime time.Time) string {
		p := filepath.Join(dir, name)
		if err := os.WriteFile(p, []byte("x"), 0o600); err != nil {
			t.Fatal(err)
		}
		os.Chtimes(p, mtime, mtime)
		return p
	}
	staleJobs := mk(".jobs.json.tmp-123", old)
	staleBak := mk(".jobs.json.bak.tmp-456", old)
	fresh := mk(".jobs.json.tmp-789", time.Now())
	real := mk("jobs.json", old)
	other := mk("notes.tmp-1", old) // no leading dot: not ours

	if n := CleanStaleTempFiles(dir, 5*time.Minute); n != 2 {
		t.Fatalf("removed %d, want 2", n)
	}
	for _, p := range []string{staleJobs, staleBak} {
		if _, err := os.Stat(p); !os.IsNotExist(err) {
			t.Errorf("%s should be removed", p)
		}
	}
	for _, p := range []string{fresh, real, other} {
		if _, err := os.Stat(p); err != nil {
			t.Errorf("%s should be kept: %v", p, err)
		}
	}
}
