package download

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// singleFileServer serves payload at /file.bin, honoring Range so the
// single-file resume path works. It does NOT advertise Accept-Ranges via
// ServeContent's defaults beyond what http.ServeContent provides.
func newSingleFileJobAndOpt(t *testing.T, payload []byte, checksum, algo string) (*Job, Options, string) {
	t.Helper()
	srv := rangeServer(t, payload)
	t.Cleanup(srv.Close)

	dir := t.TempDir()
	job := &Job{
		ID:           "single-integrity",
		URL:          srv.URL + "/file.bin",
		FileName:     "file.bin",
		TotalSize:    int64(len(payload)),
		SupportRange: true,
		Status:       StatusPending,
	}
	opt := Options{
		Out:          dir,
		Connections:  1, // force single-stream
		Downloader:   NewDownloader("test-agent", ""),
		MaxRetries:   2,
		Checksum:     checksum,
		ChecksumAlgo: algo,
	}
	return job, opt, dir
}

func TestDownloadSingleFileChecksumMismatch(t *testing.T) {
	payload := makePayload(64 * 1024) // small: single-stream path
	job, opt, _ := newSingleFileJobAndOpt(t, payload, strings.Repeat("0", 64), "sha256")

	err := DownloadSingleFile(context.Background(), opt, job, nil)
	if err == nil {
		t.Fatal("expected a checksum mismatch error, got nil")
	}
	if !errors.Is(err, ErrChecksumMismatch) {
		t.Fatalf("expected ErrChecksumMismatch, got %v", err)
	}
}

func TestDownloadSingleFileChecksumMatch(t *testing.T) {
	payload := makePayload(64 * 1024)
	good := sha256Hex(payload)
	job, opt, dir := newSingleFileJobAndOpt(t, payload, good, "sha256")

	if err := DownloadSingleFile(context.Background(), opt, job, nil); err != nil {
		t.Fatalf("expected success with correct checksum, got %v", err)
	}

	got, err := os.ReadFile(filepath.Join(dir, "file.bin"))
	if err != nil {
		t.Fatalf("read output: %v", err)
	}
	if len(got) != len(payload) {
		t.Fatalf("size mismatch: got %d want %d", len(got), len(payload))
	}
	for i := range got {
		if got[i] != payload[i] {
			t.Fatalf("byte mismatch at %d", i)
		}
	}
}

func TestDownloadSingleFileNoChecksumStillWorks(t *testing.T) {
	payload := makePayload(64 * 1024)
	job, opt, dir := newSingleFileJobAndOpt(t, payload, "", "")

	if err := DownloadSingleFile(context.Background(), opt, job, nil); err != nil {
		t.Fatalf("expected success without checksum, got %v", err)
	}
	got, err := os.ReadFile(filepath.Join(dir, "file.bin"))
	if err != nil {
		t.Fatalf("read output: %v", err)
	}
	if len(got) != len(payload) {
		t.Fatalf("size mismatch: got %d want %d", len(got), len(payload))
	}
}
