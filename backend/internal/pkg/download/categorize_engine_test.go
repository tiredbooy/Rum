package download

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/tiredbooy/Rum/backend/internal/pkg/config"
)

// TestSingleFileCategorizeMovesToDest verifies the finalize path moves a
// completed single-file download into its category directory when Categorize is
// enabled and a matching rule exists in settings.
func TestSingleFileCategorizeMovesToDest(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	outDir := t.TempDir()

	// Persist a category rule for .bin -> "Binaries", and silence notifications.
	var s config.Setting
	_ = s.LoadSettingMetadata()
	s.Silent = true
	s.OutDir = outDir
	s.EnableCategories = true
	s.Categories = []config.CategoryRule{{Name: "Binaries", Extensions: []string{".bin"}, DestDir: "Binaries"}}
	if err := s.Save(); err != nil {
		t.Fatalf("save settings: %v", err)
	}

	payload := makePayload(64 * 1024)
	srv := rangeServer(t, payload)
	defer srv.Close()

	job := &Job{
		ID:           "cat-single",
		URL:          srv.URL + "/file.bin",
		FileName:     "file.bin",
		TotalSize:    int64(len(payload)),
		SupportRange: true,
		Status:       StatusPending,
	}
	opt := Options{
		Out:         outDir,
		Connections: 1,
		MaxRetries:  1,
		Categorize:  true,
		Downloader:  NewDownloader("test-agent", ""),
	}

	if err := DownloadSingleFile(context.Background(), opt, job, nil); err != nil {
		t.Fatalf("download: %v", err)
	}

	wantPath := filepath.Join(outDir, "Binaries", "file.bin")
	if _, err := os.Stat(wantPath); err != nil {
		t.Fatalf("expected file categorized to %q: %v", wantPath, err)
	}
	if got := job.GetOutputPath(); got != wantPath {
		t.Fatalf("job.OutputPath = %q, want %q", got, wantPath)
	}
	// The original (uncategorized) location must no longer hold the file.
	if _, err := os.Stat(filepath.Join(outDir, "file.bin")); !os.IsNotExist(err) {
		t.Fatalf("expected original file to be moved away")
	}
	// Content must be intact.
	got, err := os.ReadFile(wantPath)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != len(payload) {
		t.Fatalf("size mismatch after categorize: got %d want %d", len(got), len(payload))
	}
}

// TestSegmentedCategorizeMovesToDest verifies the same for the segmented path.
func TestSegmentedCategorizeMovesToDest(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	outDir := t.TempDir()

	var s config.Setting
	_ = s.LoadSettingMetadata()
	s.Silent = true
	s.OutDir = outDir
	s.EnableCategories = true
	s.Categories = []config.CategoryRule{{Name: "Binaries", Extensions: []string{".bin"}, DestDir: "Binaries"}}
	if err := s.Save(); err != nil {
		t.Fatalf("save settings: %v", err)
	}

	payload := makePayload(minSegmentedSize * 3)
	srv := rangeServer(t, payload)
	defer srv.Close()

	job, opt, _ := newTestJobAndOpt(t, srv, payload, 6)
	opt.Out = outDir
	opt.Categorize = true

	if err := DownloadSegmented(context.Background(), opt, job, nil); err != nil {
		t.Fatalf("segmented download: %v", err)
	}

	wantPath := filepath.Join(outDir, "Binaries", "file.bin")
	if _, err := os.Stat(wantPath); err != nil {
		t.Fatalf("expected segmented file categorized to %q: %v", wantPath, err)
	}
	if got := job.GetOutputPath(); got != wantPath {
		t.Fatalf("job.OutputPath = %q, want %q", got, wantPath)
	}
	// Sidecar parts file must not linger in either location.
	if _, err := os.Stat(partsFilePath(filepath.Join(outDir, "file.bin"))); !os.IsNotExist(err) {
		t.Fatalf("parts sidecar lingered at original location")
	}
}
