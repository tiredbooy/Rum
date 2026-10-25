package handlers

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/tiredbooy/Rum/backend/internal/pkg/config"
	"github.com/tiredbooy/Rum/backend/internal/pkg/updater"
)

func updateRouter() *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.GET("/update", GetUpdateStatus)
	r.POST("/update/check", CheckUpdate)
	r.POST("/update/dismiss", DismissUpdate)
	r.POST("/update/download", DownloadUpdate)
	r.PATCH("/settings", UpdateSetting)
	return r
}

func updateCall(r *gin.Engine, method, path, body string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w
}

func TestUpdateEndpointsFlow(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	t.Setenv("RUM_CONFIG_DIR", "")
	hits := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		hits++
		_, _ = w.Write([]byte(`{"tag_name":"v9.9.9","assets":[]}`))
	}))
	defer srv.Close()

	oldSvc, oldVer := updateService, AppVersion
	defer func() { updateService, AppVersion = oldSvc, oldVer }()
	AppVersion = "1.0.0"
	updateService = &updater.Service{
		StatePath: filepath.Join(t.TempDir(), "state.json"),
		NewChecker: func(updater.Options) *updater.Checker {
			return &updater.Checker{Client: srv.Client(), APIURL: srv.URL}
		},
	}
	r := updateRouter()

	var st updater.Status
	decode := func(w *httptest.ResponseRecorder) {
		t.Helper()
		if w.Code != 200 {
			t.Fatalf("status %d: %s", w.Code, w.Body)
		}
		st = updater.Status{}
		if err := json.Unmarshal(w.Body.Bytes(), &st); err != nil {
			t.Fatal(err)
		}
	}

	decode(updateCall(r, "GET", "/update", ""))
	if st.Available || st.Current != "1.0.0" || hits != 0 {
		t.Fatalf("initial: %+v hits=%d", st, hits)
	}
	decode(updateCall(r, "POST", "/update/check", `{"auto":true}`))
	if !st.Available || st.Latest != "9.9.9" || hits != 1 {
		t.Fatalf("auto: %+v hits=%d", st, hits)
	}
	decode(updateCall(r, "POST", "/update/check", `{"auto":true}`))
	if hits != 1 {
		t.Fatalf("auto check not throttled, hits=%d", hits)
	}
	decode(updateCall(r, "POST", "/update/check", ``))
	if hits != 2 {
		t.Fatalf("manual check skipped, hits=%d", hits)
	}
	decode(updateCall(r, "POST", "/update/dismiss", `{"tag":"v9.9.9"}`))
	if !st.Dismissed {
		t.Fatalf("dismiss: %+v", st)
	}
	if w := updateCall(r, "POST", "/update/dismiss", `{"tag":"x y"}`); w.Code != 400 {
		t.Fatalf("bad tag: %d", w.Code)
	}
	// No asset in the payload: download refuses cleanly.
	if w := updateCall(r, "POST", "/update/download", ``); w.Code != 404 {
		t.Fatalf("download without asset: %d %s", w.Code, w.Body)
	}
}

func TestSettingsPatchValidatesUpdateMirrors(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	t.Setenv("RUM_CONFIG_DIR", "")
	r := updateRouter()
	if w := updateCall(r, "PATCH", "/settings", `{"update_mirrors":["javascript:alert(1)"]}`); w.Code != 400 {
		t.Fatalf("bad mirror accepted: %d", w.Code)
	}
	w := updateCall(r, "PATCH", "/settings", `{"update_mirrors":["https://ghproxy.net/"],"check_for_updates":false}`)
	if w.Code != 200 {
		t.Fatalf("good mirror rejected: %d %s", w.Code, w.Body)
	}
	var s config.Setting
	if err := s.LoadSettingMetadata(); err != nil || s.CheckForUpdates || len(s.UpdateMirrors) != 1 {
		t.Fatalf("not saved: %v %+v", err, s)
	}
}
