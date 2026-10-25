package handlers

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/tiredbooy/Rum/backend/internal/pkg/api/dto"
	"github.com/tiredbooy/Rum/backend/internal/pkg/download"
)

func doJSON(t *testing.T, h gin.HandlerFunc, method, path, id, body string) *httptest.ResponseRecorder {
	t.Helper()
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(method, path, strings.NewReader(body))
	c.Request.Header.Set("Content-Type", "application/json")
	if id != "" {
		c.Params = gin.Params{{Key: "id", Value: id}}
	}
	h(c)
	return w
}

func createOne(t *testing.T, srvURL, extra string) string {
	t.Helper()
	body := `{"urls":["` + srvURL + `/a.bin"]` + extra + `}`
	w := doJSON(t, CreateDownload, http.MethodPost, "/api/v1/downloads", "", body)
	if w.Code != http.StatusCreated {
		t.Fatalf("create status = %d (%s)", w.Code, w.Body.String())
	}
	var resp struct {
		Jobs []dto.JobInfo `json:"jobs"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil || len(resp.Jobs) != 1 {
		t.Fatalf("bad create body: %v %s", err, w.Body.String())
	}
	return resp.Jobs[0].ID
}

func TestCreateDownloadForwardsPerDownloadOptions(t *testing.T) {
	setupManager(t)
	srv := newBatchFileServer(t)
	defer srv.Close()

	id := createOne(t, srv.URL, `,"referer":"https://r.example/","user_agent":"UA/2","speed_limit":64,"max_retries":6,"dest_path":"mine"`)
	job, ok := GlobalManager.GetJob(id)
	if !ok {
		t.Fatal("job missing")
	}
	if job.GetReferer() != "https://r.example/" || job.GetUserAgent() != "UA/2" ||
		job.GetSpeedLimit() != 64 || job.GetMaxRetries() != 6 || !strings.HasSuffix(job.GetDestPath(), string(filepath.Separator)+"mine") {
		t.Fatalf("options dropped: ref=%q ua=%q speed=%d retries=%d dest=%q",
			job.GetReferer(), job.GetUserAgent(), job.GetSpeedLimit(), job.GetMaxRetries(), job.GetDestPath())
	}

	// Referer / UA must not be echoed back by the API.
	w := doJSON(t, GetDownloadStatus, http.MethodGet, "/api/v1/downloads/"+id, id, "")
	if strings.Contains(w.Body.String(), "r.example") || strings.Contains(w.Body.String(), "UA/2") {
		t.Fatalf("response leaks request headers: %s", w.Body.String())
	}
}

func TestCreateDownloadRejectsTraversingDestPath(t *testing.T) {
	setupManager(t)
	srv := newBatchFileServer(t)
	defer srv.Close()

	w := doJSON(t, CreateDownload, http.MethodPost, "/api/v1/downloads", "",
		`{"urls":["`+srv.URL+`/a.bin"],"dest_path":"../../etc"}`)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400 (%s)", w.Code, w.Body.String())
	}
	var resp dto.ErrorResponse
	_ = json.Unmarshal(w.Body.Bytes(), &resp)
	if _, ok := resp.Fields["dest_path"]; !ok {
		t.Fatalf("want a dest_path field error, got %+v", resp)
	}
}

func TestRefreshDownloadURLMatch(t *testing.T) {
	setupManager(t)
	srv := newBatchFileServer(t)
	defer srv.Close()
	id := createOne(t, srv.URL, "")
	job0, _ := GlobalManager.GetJob(id)
	job0.SetStatus("error")

	w := doJSON(t, RefreshDownloadURL, http.MethodPatch, "/api/v1/downloads/"+id+"/url", id,
		`{"url":"`+srv.URL+`/renewed.bin"}`)
	defer waitSettled(job0)
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d (%s)", w.Code, w.Body.String())
	}
	var resp struct {
		ID      string `json:"id"`
		Resumed bool   `json:"resumed"`
	}
	_ = json.Unmarshal(w.Body.Bytes(), &resp)
	if resp.ID != id || !resp.Resumed {
		t.Fatalf("resp = %+v", resp)
	}
	job, _ := GlobalManager.GetJob(id)
	if job.GetURL() != srv.URL+"/renewed.bin" {
		t.Fatalf("url = %q", job.GetURL())
	}
}

func TestRefreshDownloadURLMismatchThenRestart(t *testing.T) {
	setupManager(t)
	srv := newBatchFileServer(t) // serves 5 bytes
	defer srv.Close()
	id := createOne(t, srv.URL, "")

	job, _ := GlobalManager.GetJob(id)
	part := filepath.Join(t.TempDir(), "a.bin")
	if err := os.WriteFile(part, make([]byte, 100), 0o644); err != nil {
		t.Fatal(err)
	}
	job.SetOutputPath(part)
	job.SetTotalSize(500) // saved size differs from the 5 bytes the new link reports
	job.SetDownloaded(100)
	job.SetStatus("error")

	w := doJSON(t, RefreshDownloadURL, http.MethodPatch, "/api/v1/downloads/"+id+"/url", id,
		`{"url":"`+srv.URL+`/renewed.bin"}`)
	if w.Code != http.StatusConflict {
		t.Fatalf("status = %d, want 409 (%s)", w.Code, w.Body.String())
	}
	var er dto.ErrorResponse
	_ = json.Unmarshal(w.Body.Bytes(), &er)
	if er.Code != "remote_changed" || er.Error == "" {
		t.Fatalf("envelope = %+v", er)
	}
	if _, err := os.Stat(part); err != nil {
		t.Fatalf("partial must survive a mismatch: %v", err)
	}

	w = doJSON(t, RefreshDownloadURL, http.MethodPatch, "/api/v1/downloads/"+id+"/url", id,
		`{"url":"`+srv.URL+`/renewed.bin","restart":true}`)
	if w.Code != http.StatusOK {
		t.Fatalf("restart status = %d (%s)", w.Code, w.Body.String())
	}
	waitSettled(job)
	if !strings.Contains(w.Body.String(), `"restarted":true`) {
		t.Fatalf("body = %s", w.Body.String())
	}
}

// waitSettled blocks until a started job is no longer queued or running, so the
// test's temp dirs and server outlive its writes.
func waitSettled(job *download.Job) {
	for i := 0; i < 500; i++ {
		if s := job.GetStatus(); s != "pending" && s != "running" {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
}

func TestRefreshDownloadURLValidation(t *testing.T) {
	setupManager(t)
	for name, body := range map[string]string{
		"empty":   `{"url":""}`,
		"scheme":  `{"url":"ftp://x/y"}`,
		"garbage": `{nope`,
	} {
		w := doJSON(t, RefreshDownloadURL, http.MethodPatch, "/api/v1/downloads/x/url", "x", body)
		if w.Code != http.StatusBadRequest {
			t.Errorf("%s: status = %d, want 400 (%s)", name, w.Code, w.Body.String())
		}
	}
	w := doJSON(t, RefreshDownloadURL, http.MethodPatch, "/api/v1/downloads/missing/url", "missing", `{"url":"http://a.example/x"}`)
	if w.Code != http.StatusNotFound {
		t.Errorf("unknown job: status = %d, want 404 (%s)", w.Code, w.Body.String())
	}
}
