package handlers

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/tiredbooy/Rum/backend/internal/pkg/download"
)

func TestImportExportEndpointsRoundtrip(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	GlobalManager = download.NewJobManager(&download.Options{Parallel: 1, Out: t.TempDir()})
	t.Cleanup(GlobalManager.Shutdown)

	post := func(body string) *httptest.ResponseRecorder {
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request = httptest.NewRequest(http.MethodPost, "/api/v1/downloads/import", strings.NewReader(body))
		c.Request.Header.Set("Content-Type", "application/json")
		ImportDownloads(c)
		return w
	}

	if w := post(`{"version":9,"downloads":[]}`); w.Code != http.StatusBadRequest {
		t.Fatalf("unknown version must be 400, got %d", w.Code)
	}
	if w := post(`nope`); w.Code != http.StatusBadRequest {
		t.Fatalf("bad json must be 400, got %d", w.Code)
	}
	w := post(`{"version":1,"downloads":[{"url":"http://x.test/a.zip"},{"url":"bad"}]}`)
	var res download.ImportResult
	_ = json.Unmarshal(w.Body.Bytes(), &res)
	if w.Code != 200 || res.Added != 1 || res.Invalid != 1 {
		t.Fatalf("import: %d %s", w.Code, w.Body.String())
	}

	ew := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(ew)
	c.Request = httptest.NewRequest(http.MethodGet, "/api/v1/downloads/export", nil)
	ExportDownloads(c)
	if !strings.Contains(ew.Body.String(), "http://x.test/a.zip") || !strings.Contains(ew.Header().Get("Content-Disposition"), "attachment") {
		t.Fatalf("export: %v %s", ew.Header(), ew.Body.String())
	}
}

// A cross-site form/text POST needs no CORS preflight; requiring a JSON content
// type forces one, so another web page cannot inject downloads.
func TestImportRejectsNonJSONContentType(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	GlobalManager = download.NewJobManager(&download.Options{Parallel: 1, Out: t.TempDir()})
	t.Cleanup(GlobalManager.Shutdown)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPost, "/api/v1/downloads/import",
		strings.NewReader(`{"version":1,"downloads":[{"url":"http://x.test/a.zip"}]}`))
	c.Request.Header.Set("Content-Type", "text/plain")
	ImportDownloads(c)
	if w.Code != http.StatusUnsupportedMediaType {
		t.Fatalf("want 415, got %d", w.Code)
	}
	if n := len(GlobalManager.GetAllJobs()); n != 0 {
		t.Fatalf("nothing may be imported, got %d jobs", n)
	}
}
