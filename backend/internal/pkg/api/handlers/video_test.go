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
	"github.com/tiredbooy/Rum/backend/internal/pkg/api/dto"
	"github.com/tiredbooy/Rum/backend/internal/pkg/config"
	"github.com/tiredbooy/Rum/backend/internal/pkg/download"
	"github.com/tiredbooy/Rum/backend/internal/pkg/ytdlp/fakeytdlp"
)

// videoAPI installs a manager whose yt-dlp is this test binary as the fake.
func videoAPI(t *testing.T, mode string) {
	t.Helper()
	freshManager(t, &download.Options{Parallel: 1, Out: t.TempDir()})
	t.Setenv(fakeytdlp.Env, "1")
	t.Setenv(fakeytdlp.ModeEnv, mode)
	// A stub ffmpeg on PATH, so merged formats (h720…) are offered whether or
	// not the machine has a real one (CI runners don't).
	bin := t.TempDir()
	if err := os.WriteFile(filepath.Join(bin, "ffmpeg"), []byte("#!/bin/sh\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))
	var s config.Setting
	s.OutDir = t.TempDir()
	s.YtdlpPath = os.Args[0]
	GlobalManager.ApplySettings(s)
	videoInfos.mu.Lock()
	videoInfos.m = map[string]cachedInfo{}
	videoInfos.mu.Unlock()
}

func postJSON(h gin.HandlerFunc, body, contentType string) *httptest.ResponseRecorder {
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPost, "/x", strings.NewReader(body))
	c.Request.Header.Set("Content-Type", contentType)
	h(c)
	return w
}

func TestVideoInfoListsPickerOptions(t *testing.T) {
	videoAPI(t, "")
	w := postJSON(GetVideoInfo, `{"url":"https://example.com/watch?v=1"}`, "application/json")
	if w.Code != http.StatusOK {
		t.Fatalf("status %d: %s", w.Code, w.Body)
	}
	var got videoInfoResponse
	if err := json.Unmarshal(w.Body.Bytes(), &got); err != nil {
		t.Fatal(err)
	}
	if got.Title != "Test Video: 100% real/fake" || len(got.Options) == 0 || got.Options[0].Key != "best" {
		t.Fatalf("response: %+v", got)
	}
}

func TestVideoInfoErrors(t *testing.T) {
	videoAPI(t, "unavailable")
	w := postJSON(GetVideoInfo, `{"url":"https://example.com/v"}`, "application/json")
	var e dto.ErrorResponse
	_ = json.Unmarshal(w.Body.Bytes(), &e)
	if w.Code != http.StatusUnprocessableEntity || e.Code != "video_unavailable" || e.Error == "" {
		t.Fatalf("unavailable: %d %+v", w.Code, e)
	}

	// no tool anywhere -> 424 ytdlp_missing
	var s config.Setting
	GlobalManager.ApplySettings(s)
	t.Setenv("PATH", t.TempDir())
	w = postJSON(GetVideoInfo, `{"url":"https://example.com/v"}`, "application/json")
	_ = json.Unmarshal(w.Body.Bytes(), &e)
	if w.Code != http.StatusFailedDependency || e.Code != "ytdlp_missing" {
		t.Fatalf("missing: %d %+v", w.Code, e)
	}
}

func TestVideoEndpointsRejectBadInput(t *testing.T) {
	videoAPI(t, "")
	cases := []struct {
		name, body, ct string
		want           int
	}{
		{"form post", `url=https://example.com/v`, "application/x-www-form-urlencoded", http.StatusUnsupportedMediaType},
		{"file scheme", `{"url":"file:///etc/passwd"}`, "application/json", http.StatusBadRequest},
		{"option-looking url", `{"url":"--exec=id"}`, "application/json", http.StatusBadRequest},
		{"bad json", `{nope`, "application/json", http.StatusBadRequest},
	}
	for _, tc := range cases {
		for name, h := range map[string]gin.HandlerFunc{"info": GetVideoInfo, "create": CreateVideoDownload, "ext": ExtVideo} {
			if w := postJSON(h, tc.body, tc.ct); w.Code != tc.want {
				t.Errorf("%s/%s: status %d, want %d (%s)", name, tc.name, w.Code, tc.want, w.Body)
			}
		}
	}
}

func TestCreateVideoDownloadQueuesChosenFormat(t *testing.T) {
	videoAPI(t, "")
	w := postJSON(CreateVideoDownload, `{"url":"https://example.com/v","format":"h720","auto_start":false}`, "application/json")
	if w.Code != http.StatusCreated {
		t.Fatalf("status %d: %s", w.Code, w.Body)
	}
	var resp struct {
		Jobs []dto.JobInfo `json:"jobs"`
	}
	_ = json.Unmarshal(w.Body.Bytes(), &resp)
	if len(resp.Jobs) != 1 {
		t.Fatalf("jobs: %s", w.Body)
	}
	job, ok := GlobalManager.GetJob(resp.Jobs[0].ID)
	if !ok || !job.IsVideo() || job.GetVideoFormat() != "h720" || job.GetStatus() != download.StatusPending {
		t.Fatalf("job not queued as expected: %+v", job)
	}

	// A format the page does not offer is a field error, not a job.
	w = postJSON(CreateVideoDownload, `{"url":"https://example.com/other","format":"h1080"}`, "application/json")
	if w.Code != http.StatusBadRequest {
		t.Fatalf("unoffered format: %d %s", w.Code, w.Body)
	}
	w = postJSON(CreateVideoDownload, `{"url":"https://example.com/other","format":"best; rm -rf /"}`, "application/json")
	if w.Code != http.StatusBadRequest {
		t.Fatalf("injected format: %d %s", w.Code, w.Body)
	}
}

func TestYtdlpStatus(t *testing.T) {
	videoAPI(t, "")
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "/api/v1/ytdlp", nil)
	GetYtdlpStatus(c)
	var st ytdlpStatusResponse
	_ = json.Unmarshal(w.Body.Bytes(), &st)
	if !st.Found || st.Source != "configured" || st.Version != "2025.01.01" {
		t.Fatalf("status: %+v", st)
	}
}

func TestSettingsValidateYtdlpPath(t *testing.T) {
	freshManager(t, &download.Options{Parallel: 1, Out: t.TempDir()})
	w := patchSettings(t, `{"ytdlp_path":"/no/such/yt-dlp"}`)
	if w.Code != http.StatusBadRequest || decodeError(t, w).Fields["ytdlp_path"] == "" {
		t.Fatalf("missing file accepted: %d %s", w.Code, w.Body)
	}
	if w = patchSettings(t, `{"ytdlp_path":""}`); w.Code != http.StatusOK {
		t.Fatalf("clearing the path failed: %d %s", w.Code, w.Body)
	}
	if w = patchSettings(t, `{"ytdlp_path":"`+os.Args[0]+`"}`); w.Code != http.StatusOK {
		t.Fatalf("real file rejected: %d %s", w.Code, w.Body)
	}
}
