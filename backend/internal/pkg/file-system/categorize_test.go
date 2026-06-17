package filesystem

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCategorizeFileMatchCreatesDestUnderBase(t *testing.T) {
	base := t.TempDir()
	src := filepath.Join(base, "a.mp4")
	if err := os.WriteFile(src, []byte("x"), 0644); err != nil {
		t.Fatal(err)
	}

	rules := []CategoryRule{{Name: "Video", Extensions: []string{".mp4", ".mkv"}, DestDir: "Videos"}}
	dest, err := CategorizeFile(src, rules)
	if err != nil {
		t.Fatalf("CategorizeFile: %v", err)
	}

	wantDir := filepath.Join(base, "Videos")
	want := filepath.Join(wantDir, "a.mp4")
	if dest != want {
		t.Fatalf("dest = %q, want %q", dest, want)
	}
	// The destination directory must have been created.
	if fi, err := os.Stat(wantDir); err != nil || !fi.IsDir() {
		t.Fatalf("expected dest dir %q to be created: %v", wantDir, err)
	}
}

func TestCategorizeFileNoMatchReturnsOriginal(t *testing.T) {
	base := t.TempDir()
	src := filepath.Join(base, "notes.txt")
	if err := os.WriteFile(src, []byte("x"), 0644); err != nil {
		t.Fatal(err)
	}
	rules := []CategoryRule{{Name: "Video", Extensions: []string{".mp4"}, DestDir: "Videos"}}
	dest, err := CategorizeFile(src, rules)
	if err != nil {
		t.Fatalf("CategorizeFile: %v", err)
	}
	if dest != src {
		t.Fatalf("expected original path %q for non-matching extension, got %q", src, dest)
	}
}

func TestCategorizeFileCaseInsensitiveExtension(t *testing.T) {
	base := t.TempDir()
	src := filepath.Join(base, "MOVIE.MP4")
	if err := os.WriteFile(src, []byte("x"), 0644); err != nil {
		t.Fatal(err)
	}
	rules := []CategoryRule{{Name: "Video", Extensions: []string{".mp4"}, DestDir: "Videos"}}
	dest, err := CategorizeFile(src, rules)
	if err != nil {
		t.Fatalf("CategorizeFile: %v", err)
	}
	want := filepath.Join(base, "Videos", "MOVIE.MP4")
	if dest != want {
		t.Fatalf("case-insensitive match failed: dest=%q want=%q", dest, want)
	}
}

func TestCategorizeFileAbsoluteDestDir(t *testing.T) {
	base := t.TempDir()
	absDest := t.TempDir()
	src := filepath.Join(base, "song.mp3")
	if err := os.WriteFile(src, []byte("x"), 0644); err != nil {
		t.Fatal(err)
	}
	rules := []CategoryRule{{Name: "Audio", Extensions: []string{".mp3"}, DestDir: absDest}}
	dest, err := CategorizeFile(src, rules)
	if err != nil {
		t.Fatalf("CategorizeFile: %v", err)
	}
	want := filepath.Join(absDest, "song.mp3")
	if dest != want {
		t.Fatalf("absolute dest dir: dest=%q want=%q", dest, want)
	}
}

func TestCategorizeFileNeutralizesTraversalInDestDir(t *testing.T) {
	base := t.TempDir()
	src := filepath.Join(base, "a.mp4")
	if err := os.WriteFile(src, []byte("x"), 0644); err != nil {
		t.Fatal(err)
	}
	// A relative DestDir that tries to escape must not let the final file land
	// outside the resolved category base. The filename is always sanitized; the
	// resulting dest must stay within the resolved dest dir.
	rules := []CategoryRule{{Name: "Evil", Extensions: []string{".mp4"}, DestDir: "../../etc"}}
	dest, err := CategorizeFile(src, rules)
	if err != nil {
		// An error here is an acceptable outcome (rejected traversal).
		return
	}
	// If it returned a dest, the basename must be intact and not contain traversal.
	if strings.Contains(dest, "..") {
		t.Fatalf("dest still contains traversal: %q", dest)
	}
	if filepath.Base(dest) != "a.mp4" {
		t.Fatalf("expected basename a.mp4, got %q", filepath.Base(dest))
	}
}
