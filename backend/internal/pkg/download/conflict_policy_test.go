package download

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/tiredbooy/Rum/backend/internal/pkg/config"
)

// setConflictPolicy persists the file-conflict policy (and keeps the test's
// Silent/OutDir) so applyConflictPolicy, which reads settings fresh, sees it.
func setConflictPolicy(t *testing.T, outDir, policy string) {
	t.Helper()
	var s config.Setting
	_ = s.LoadSettingMetadata()
	s.Silent = true
	s.OutDir = outDir
	s.FileConflict = policy
	if err := s.Save(); err != nil {
		t.Fatalf("persist policy %q: %v", policy, err)
	}
}

// TestApplyConflictPolicy guards A5: a normal download must honor the
// overwrite/skip/rename policy instead of silently clobbering an existing file.
func TestApplyConflictPolicy(t *testing.T) {
	m, outDir := newManagerForTest(t, 1)
	defer m.Shutdown()

	job := &Job{URL: "http://x/file.bin", FileName: "file.bin", ContentType: "application/octet-stream"}
	dest := PrepareOutputPath(*m.opt, job.FileName, job.URL, job.ContentType)
	if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(dest, []byte("existing"), 0o644); err != nil {
		t.Fatal(err)
	}

	t.Run("rename bumps the name", func(t *testing.T) {
		setConflictPolicy(t, outDir, "rename")
		name, skip := m.applyConflictPolicy(job)
		if skip {
			t.Fatal("rename must not skip")
		}
		if name == job.FileName || !strings.Contains(name, "(1)") {
			t.Fatalf("rename should yield a (1) name, got %q", name)
		}
	})

	t.Run("skip drops the job", func(t *testing.T) {
		setConflictPolicy(t, outDir, "skip")
		if _, skip := m.applyConflictPolicy(job); !skip {
			t.Fatal("skip policy must skip when the file exists")
		}
	})

	t.Run("overwrite keeps the name and does not delete at creation", func(t *testing.T) {
		setConflictPolicy(t, outDir, "overwrite")
		name, skip := m.applyConflictPolicy(job)
		if skip || name != job.FileName {
			t.Fatalf("overwrite should keep %q, got name=%q skip=%v", job.FileName, name, skip)
		}
		if _, err := os.Stat(dest); err != nil {
			t.Fatal("overwrite must NOT remove the existing file at creation time")
		}
	})

	t.Run("no conflict leaves the name unchanged", func(t *testing.T) {
		setConflictPolicy(t, outDir, "rename")
		fresh := &Job{URL: "http://x/free.bin", FileName: "free-nonexistent.bin", ContentType: "application/octet-stream"}
		name, skip := m.applyConflictPolicy(fresh)
		if skip || name != fresh.FileName {
			t.Fatalf("no-conflict should keep %q, got name=%q skip=%v", fresh.FileName, name, skip)
		}
	})
}
