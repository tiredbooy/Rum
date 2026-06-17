package handlers

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/tiredbooy/Rum/backend/internal/pkg/api/dto"
	"github.com/tiredbooy/Rum/backend/internal/pkg/config"
	"github.com/tiredbooy/Rum/backend/internal/pkg/download"
)

// fileServer serves a small body with range support so HEAD probes succeed.
func newBatchFileServer(t *testing.T) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Accept-Ranges", "bytes")
		w.Header().Set("Content-Length", "5")
		if r.Method == http.MethodHead {
			w.WriteHeader(http.StatusOK)
			return
		}
		w.Write([]byte("hello"))
	}))
	return srv
}

// setupManager initializes GlobalManager against isolated config/out dirs.
func setupManager(t *testing.T) {
	t.Helper()
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	outDir := t.TempDir()
	var s config.Setting
	_ = s.LoadSettingMetadata()
	s.Silent = true
	s.OutDir = outDir
	if err := s.Save(); err != nil {
		t.Fatalf("save settings: %v", err)
	}
	opt := &download.Options{Parallel: 1, Connections: 1, Out: outDir, Silent: true}
	InitAPI(opt)
	t.Cleanup(func() {
		if GlobalManager != nil {
			GlobalManager.Shutdown()
		}
	})
}

func TestCreateBatchPartialErrors(t *testing.T) {
	setupManager(t)
	srv := newBatchFileServer(t)
	defer srv.Close()

	body := `{"urls":["` + srv.URL + `/a.bin","` + srv.URL + `/b.bin","ftp://bad/host"]}`

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPost, "/api/v1/downloads/batch", strings.NewReader(body))
	c.Request.Header.Set("Content-Type", "application/json")

	CreateBatch(c)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 (body: %s)", w.Code, w.Body.String())
	}
	var resp dto.BatchCreateResultResponse
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal: %v (%s)", err, w.Body.String())
	}
	if len(resp.Created) != 2 {
		t.Fatalf("created = %d, want 2 (%+v)", len(resp.Created), resp.Created)
	}
	if len(resp.Errors) != 1 {
		t.Fatalf("errors = %d, want 1 (%+v)", len(resp.Errors), resp.Errors)
	}
	if !strings.HasPrefix(resp.Errors[0].URL, "ftp://") {
		t.Fatalf("expected the ftp URL in errors, got %q", resp.Errors[0].URL)
	}
}

func TestCreateBatchValidationEmptyURLs(t *testing.T) {
	setupManager(t)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPost, "/api/v1/downloads/batch", strings.NewReader(`{"urls":[]}`))
	c.Request.Header.Set("Content-Type", "application/json")

	CreateBatch(c)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400 (body: %s)", w.Code, w.Body.String())
	}
}

// TestCreateDownloadBackwardCompatNoNewFields verifies the existing create
// endpoint still works when the body has NONE of the new optional fields.
func TestCreateDownloadBackwardCompatNoNewFields(t *testing.T) {
	setupManager(t)
	srv := newBatchFileServer(t)
	defer srv.Close()

	body := `{"urls":["` + srv.URL + `/legacy.bin"]}`

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPost, "/api/v1/downloads", strings.NewReader(body))
	c.Request.Header.Set("Content-Type", "application/json")

	CreateDownload(c)

	if w.Code != http.StatusCreated {
		t.Fatalf("status = %d, want 201 (body: %s)", w.Code, w.Body.String())
	}
	// Response shape must remain { "jobs": [...] }.
	var resp struct {
		Jobs []dto.JobInfo `json:"jobs"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal: %v (%s)", err, w.Body.String())
	}
	if len(resp.Jobs) != 1 {
		t.Fatalf("jobs = %d, want 1", len(resp.Jobs))
	}
}

// TestCreateDownloadRejectsBadChecksumAlgo verifies the new optional field
// validation rejects an unsupported algo at the edge.
func TestCreateDownloadRejectsBadChecksumAlgo(t *testing.T) {
	setupManager(t)
	srv := newBatchFileServer(t)
	defer srv.Close()

	body := `{"urls":["` + srv.URL + `/x.bin"],"checksum":"abc","checksum_algo":"crc32"}`
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPost, "/api/v1/downloads", strings.NewReader(body))
	c.Request.Header.Set("Content-Type", "application/json")

	CreateDownload(c)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400 (body: %s)", w.Code, w.Body.String())
	}
	var resp dto.ErrorResponse
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if _, ok := resp.Fields["checksum_algo"]; !ok {
		t.Fatalf("expected checksum_algo field error, got %v", resp.Fields)
	}
}
