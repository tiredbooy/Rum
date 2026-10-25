package handlers

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/tiredbooy/Rum/backend/internal/pkg/download"
)

func TestDownloadResponseCarriesChecksumResult(t *testing.T) {
	j := &download.Job{ID: "a", Status: download.StatusCompleted}
	b, _ := json.Marshal(toDownloadResponseFull(j))
	if strings.Contains(string(b), "checksum_status") {
		t.Fatalf("not-available must add nothing to the payload: %s", b)
	}

	j.SetChecksumResult(download.ChecksumMismatch, "sha256", download.ChecksumSourceSidecar)
	for _, resp := range []any{toDownloadResponseFull(j), toDownloadResponse(j)} {
		b, _ = json.Marshal(resp)
		for _, want := range []string{`"checksum_status":"mismatch"`, `"checksum_result_algo":"sha256"`, `"checksum_source":"sidecar"`} {
			if !strings.Contains(string(b), want) {
				t.Fatalf("%s missing %s", b, want)
			}
		}
	}
}

func TestChecksumEndpoints(t *testing.T) {
	setupManager(t)
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.POST("/downloads/:id/verify-checksum", VerifyDownloadChecksum)
	r.POST("/downloads/:id/redownload", RedownloadDownload)

	p := filepath.Join(t.TempDir(), "a.bin")
	if err := os.WriteFile(p, []byte("hello"), 0o644); err != nil {
		t.Fatal(err)
	}
	srv := newBatchFileServer(t)
	defer srv.Close()
	jobs, err := GlobalManager.CreateJobsFromURLsWithOptions([]string{srv.URL + "/a.bin"},
		download.JobCreateOptions{Checksum: strings.Repeat("0", 64), ChecksumAlgo: "sha256"})
	if err != nil || len(jobs) != 1 {
		t.Fatalf("create: %v %v", jobs, err)
	}
	job := jobs[0]
	job.SetOutputPath(p)
	job.SetStatus(download.StatusCompleted)

	post := func(path string) *httptest.ResponseRecorder {
		w := httptest.NewRecorder()
		r.ServeHTTP(w, httptest.NewRequest(http.MethodPost, path, nil))
		return w
	}

	w := post("/downloads/" + job.ID + "/verify-checksum")
	if w.Code != http.StatusOK || !strings.Contains(w.Body.String(), `"status":"mismatch"`) {
		t.Fatalf("verify: %d %s", w.Code, w.Body)
	}
	if _, err := os.Stat(p); err != nil {
		t.Fatalf("verify must not delete the file: %v", err)
	}
	if w := post("/downloads/nope/verify-checksum"); w.Code != http.StatusNotFound {
		t.Fatalf("unknown id: %d", w.Code)
	}

	w = post("/downloads/" + job.ID + "/redownload")
	if w.Code != http.StatusAccepted {
		t.Fatalf("redownload: %d %s", w.Code, w.Body)
	}
	if _, err := os.Stat(p); !os.IsNotExist(err) {
		t.Fatalf("redownload should replace the damaged file, stat err = %v", err)
	}
	if w := post("/downloads/nope/redownload"); w.Code != http.StatusNotFound {
		t.Fatalf("unknown id: %d", w.Code)
	}
}
