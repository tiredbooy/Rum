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
)

// patchSettings runs UpdateSetting against an isolated config dir and returns
// the recorder.
func patchSettings(t *testing.T, body string) *httptest.ResponseRecorder {
	t.Helper()
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPatch, "/api/v1/settings", strings.NewReader(body))
	c.Request.Header.Set("Content-Type", "application/json")
	UpdateSetting(c)
	return w
}

func decodeError(t *testing.T, w *httptest.ResponseRecorder) dto.ErrorResponse {
	t.Helper()
	var got dto.ErrorResponse
	if err := json.Unmarshal(w.Body.Bytes(), &got); err != nil {
		t.Fatalf("unmarshal error body: %v (%s)", err, w.Body.String())
	}
	return got
}

// Every invalid enum / proxy value must come back as a NAMED field error so the
// settings form can show it inline. They used to be accepted with a 200 and then
// silently rewritten to the default by config.Validate, which looked to the user
// like a control that refused to change.
func TestUpdateSettingRejectsInvalidValuesPerField(t *testing.T) {
	cases := []struct {
		name  string
		body  string
		field string
	}{
		{"theme", `{"preferred_theme":"midnight"}`, "preferred_theme"},
		{"density", `{"ui_density":"roomy"}`, "ui_density"},
		{"conflict", `{"file_confilict":"clobber"}`, "file_confilict"},
		{"action", `{"post_download":{"action":"selfdestruct"}}`, "post_download.action"},
		{"accent", `{"accent_color":"not-a-color"}`, "accent_color"},
		{"proxy", `{"proxy":"ftp://nope:21"}`, "proxy"},
		{"connections", `{"connections":99}`, "connections"},
		{"parallel", `{"max_parallel":0}`, "max_parallel"},
		{"retries", `{"max_retries":-1}`, "max_retries"},
		{"backoff", `{"retry_backoff_sec":0}`, "retry_backoff_sec"},
		{"speed", `{"speed_limit_kb":-5}`, "speed_limit_kb"},
		{"log level", `{"log_level":"verbose"}`, "log_level"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv("XDG_CONFIG_HOME", t.TempDir())
			w := patchSettings(t, tc.body)
			if w.Code != http.StatusBadRequest {
				t.Fatalf("status = %d, want 400 (%s)", w.Code, w.Body.String())
			}
			got := decodeError(t, w)
			if got.Fields[tc.field] == "" {
				t.Fatalf("no field error for %q: %+v", tc.field, got.Fields)
			}
		})
	}
}

func TestUpdateSettingAcceptsValidValues(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	w := patchSettings(t, `{"preferred_theme":"dark","ui_density":"compact","accent_color":"#1a2b3c","proxy":"127.0.0.1:1080","connections":4}`)
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 (%s)", w.Code, w.Body.String())
	}

	var got map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &got); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if got["preferred_theme"] != "dark" {
		t.Errorf("preferred_theme = %v, want dark", got["preferred_theme"])
	}
	if got["proxy"] != "http://127.0.0.1:1080" {
		t.Errorf("proxy = %v, want the normalized http URL", got["proxy"])
	}
}

// A download folder that cannot be used must be reported at save time, not
// discovered when a download fails an hour later.
func TestUpdateSettingRejectsUnusableOutDir(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())

	// A regular file is not a folder.
	file := filepath.Join(t.TempDir(), "not-a-dir")
	if err := os.WriteFile(file, []byte("x"), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	body, _ := json.Marshal(map[string]string{"out_dir": file})

	w := patchSettings(t, string(body))
	if w.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400 (%s)", w.Code, w.Body.String())
	}
	if decodeError(t, w).Fields["out_dir"] == "" {
		t.Fatal("expected an out_dir field error")
	}
}

func TestUpdateSettingRejectsRelativeTempDir(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	w := patchSettings(t, `{"temp_dir":"relative/path"}`)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400 (%s)", w.Code, w.Body.String())
	}
	if decodeError(t, w).Fields["temp_dir"] == "" {
		t.Fatal("expected a temp_dir field error")
	}
}

// An absolute directory that does not exist yet is created rather than rejected —
// picking a fresh folder in the UI should just work.
func TestUpdateSettingCreatesMissingOutDir(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	target := filepath.Join(t.TempDir(), "new", "downloads")
	body, _ := json.Marshal(map[string]string{"out_dir": target})

	w := patchSettings(t, string(body))
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 (%s)", w.Code, w.Body.String())
	}
	if info, err := os.Stat(target); err != nil || !info.IsDir() {
		t.Fatalf("out_dir was not created: %v", err)
	}
}

// An empty temp_dir is the documented "write next to the final file" default and
// must stay accepted.
func TestUpdateSettingAcceptsEmptyTempDir(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	w := patchSettings(t, `{"temp_dir":""}`)
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 (%s)", w.Code, w.Body.String())
	}
}

// writeCorruptSettings drops an unparseable settings.json into an isolated
// config dir.
func writeCorruptSettings(t *testing.T) {
	t.Helper()
	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", dir)
	path := filepath.Join(dir, "rum", "settings.json")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(path, []byte("this is not json {{{"), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
}

// A corrupt settings.json is repaired in place by LoadSettingMetadata, which
// still returns an error describing what it recovered from. Treating that as
// fatal made GET /settings answer 500 and the settings page show "Could not
// load preferences" — for settings it was in fact holding.
func TestGetSettingsRecoversFromACorruptFile(t *testing.T) {
	writeCorruptSettings(t)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "/api/v1/settings", nil)
	GetSettings(c)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 (%s)", w.Code, w.Body.String())
	}
	var got map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &got); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if got["preferred_theme"] != "system" || got["verify_integrity"] != true {
		t.Fatalf("defaults not returned after recovery: %v", got)
	}
}

// The same recovery path must not break a save either.
func TestUpdateSettingRecoversFromACorruptFile(t *testing.T) {
	writeCorruptSettings(t)

	w := patchSettings(t, `{"preferred_theme":"dark"}`)
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 (%s)", w.Code, w.Body.String())
	}
}

// Schedule and category reads share the loader, so they recover too.
func TestScheduleAndCategoriesRecoverFromACorruptFile(t *testing.T) {
	writeCorruptSettings(t)

	for name, handler := range map[string]gin.HandlerFunc{
		"schedule":   GetSchedule,
		"categories": GetCategories,
	} {
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request = httptest.NewRequest(http.MethodGet, "/api/v1/settings/"+name, nil)
		handler(c)
		if w.Code != http.StatusOK {
			t.Fatalf("%s status = %d, want 200 (%s)", name, w.Code, w.Body.String())
		}
	}
}

// log_level is settable from the settings UI now, so the endpoint must accept
// the valid levels and round-trip them.
func TestUpdateSettingAcceptsLogLevel(t *testing.T) {
	for _, level := range []string{"debug", "info", "warn", "error"} {
		t.Run(level, func(t *testing.T) {
			t.Setenv("XDG_CONFIG_HOME", t.TempDir())
			w := patchSettings(t, `{"log_level":"`+level+`"}`)
			if w.Code != http.StatusOK {
				t.Fatalf("status = %d, want 200 (%s)", w.Code, w.Body.String())
			}
			var got map[string]any
			if err := json.Unmarshal(w.Body.Bytes(), &got); err != nil {
				t.Fatalf("unmarshal: %v", err)
			}
			if got["log_level"] != level {
				t.Fatalf("log_level = %v, want %s", got["log_level"], level)
			}
		})
	}
}

// The deprecated start_on_launch key is still accepted over the wire and lands
// on the field that actually drives behaviour.
func TestUpdateSettingRedirectsLegacyStartOnLaunch(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())

	w := patchSettings(t, `{"start_on_launch":false}`)
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 (%s)", w.Code, w.Body.String())
	}
	var got map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &got); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if got["auto_resume_on_launch"] != false {
		t.Fatalf("auto_resume_on_launch = %v, want false", got["auto_resume_on_launch"])
	}
	if got["start_on_launch"] != false {
		t.Fatalf("start_on_launch = %v, want it mirrored to false", got["start_on_launch"])
	}
}
