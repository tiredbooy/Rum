package filesystem

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestAtomicWriteFile(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "jobs.json")

	if err := AtomicWriteFile(target, []byte(`{"a":1}`), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	got, err := os.ReadFile(target)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != `{"a":1}` {
		t.Fatalf("content mismatch: %q", got)
	}

	// Overwrite atomically.
	if err := AtomicWriteFile(target, []byte(`{"b":2}`), 0o644); err != nil {
		t.Fatalf("rewrite: %v", err)
	}
	got, _ = os.ReadFile(target)
	if string(got) != `{"b":2}` {
		t.Fatalf("rewrite content mismatch: %q", got)
	}

	// No leftover temp files in the directory.
	entries, _ := os.ReadDir(dir)
	for _, e := range entries {
		if strings.Contains(e.Name(), ".tmp-") {
			t.Fatalf("leftover temp file: %s", e.Name())
		}
	}
}

func TestAtomicWriteCreatesDir(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "nested", "deep", "f.json")
	if err := AtomicWriteFile(target, []byte("x"), 0o644); err != nil {
		t.Fatalf("write into missing dir: %v", err)
	}
	if _, err := os.Stat(target); err != nil {
		t.Fatal(err)
	}
}

func TestSanitizeFileName(t *testing.T) {
	cases := map[string]string{
		"normal.zip":             "normal.zip",
		"../../etc/passwd":       "passwd",
		"/abs/path/file.bin":     "file.bin",
		`..\..\windows\sys.dll`:  "sys.dll",
		`C:\Users\x\evil.exe`:    "evil.exe",
		"":                       "download",
		"..":                     "download",
		".":                      "download",
		"file:with*bad?chars":   "filewithbadchars",
		"  spaced  .txt  ":       "spaced  .txt",
		"trailing...":            "trailing",
		"con/../../secret":       "secret",
	}
	for in, want := range cases {
		if got := SanitizeFileName(in); got != want {
			t.Errorf("SanitizeFileName(%q) = %q, want %q", in, got, want)
		}
	}
	// Result must never contain a separator.
	for _, in := range []string{"../../x", `a\b\c`, "/x/y/z"} {
		if strings.ContainsAny(SanitizeFileName(in), `/\`) {
			t.Errorf("sanitized %q still has a separator", in)
		}
	}
}

func TestSafeJoin(t *testing.T) {
	base := t.TempDir()

	full, err := SafeJoin(base, "report.pdf")
	if err != nil {
		t.Fatal(err)
	}
	if filepath.Dir(full) != base {
		t.Fatalf("not under base: %s", full)
	}

	// Traversal is neutralized to a basename, stays under base.
	full, err = SafeJoin(base, "../../../../etc/passwd")
	if err != nil {
		t.Fatalf("traversal should be sanitized not error: %v", err)
	}
	if filepath.Base(full) != "passwd" || filepath.Dir(full) != base {
		t.Fatalf("traversal escaped: %s", full)
	}
}
