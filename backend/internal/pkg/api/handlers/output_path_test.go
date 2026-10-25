package handlers

import (
	"testing"

	"github.com/tiredbooy/Rum/backend/internal/pkg/download"
)

func TestToDownloadResponseIncludesOutputPath(t *testing.T) {
	job := &download.Job{ID: "j1", OutputPath: "/data/a.bin"}
	if got := toDownloadResponse(job).OutputPath; got != "/data/a.bin" {
		t.Fatalf("OutputPath = %q", got)
	}
	// The list response must not leak filesystem paths.
	if got := toDownloadResponseFull(job).OutputPath; got != "" {
		t.Fatalf("list response leaked OutputPath %q", got)
	}
}
