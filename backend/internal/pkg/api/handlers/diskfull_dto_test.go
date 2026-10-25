package handlers

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/tiredbooy/Rum/backend/internal/pkg/download"
)

func TestDownloadResponseCarriesDiskFullDetails(t *testing.T) {
	job := &download.Job{ID: "a", Status: download.StatusError,
		Error: &download.DiskFullError{NeededBytes: 5 << 30, FreeBytes: 2 << 30, Path: "/data"}}
	data, err := json.Marshal(toDownloadResponseFull(job))
	if err != nil {
		t.Fatal(err)
	}
	var got map[string]any
	_ = json.Unmarshal(data, &got)
	if got["error_code"] != "disk_full" || got["needed_bytes"] != float64(5<<30) ||
		got["free_bytes"] != float64(2<<30) || got["disk_path"] != "/data" {
		t.Fatalf("missing disk-full fields: %s", data)
	}

	plain, _ := json.Marshal(toDownloadResponseFull(&download.Job{ID: "b", Status: download.StatusRunning}))
	if strings.Contains(string(plain), "error_code") || strings.Contains(string(plain), "needed_bytes") {
		t.Fatalf("fields must be omitted when unset: %s", plain)
	}
}
