package handlers

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/tiredbooy/Rum/backend/internal/pkg/api/dto"
)

func TestScheduleGetPutRoundTrip(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())

	// PUT a schedule.
	put := `{"scheduled_start_enabled":true,"rules":[{"start_hour":9,"end_hour":18,"limit_kbps":500,"days":[1,2,3,4,5]}]}`
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPut, "/api/v1/settings/schedule", strings.NewReader(put))
	c.Request.Header.Set("Content-Type", "application/json")
	PutSchedule(c)
	if w.Code != http.StatusOK {
		t.Fatalf("PUT status = %d, want 200 (%s)", w.Code, w.Body.String())
	}

	// GET it back.
	w2 := httptest.NewRecorder()
	c2, _ := gin.CreateTestContext(w2)
	c2.Request = httptest.NewRequest(http.MethodGet, "/api/v1/settings/schedule", nil)
	GetSchedule(c2)
	if w2.Code != http.StatusOK {
		t.Fatalf("GET status = %d, want 200 (%s)", w2.Code, w2.Body.String())
	}
	var got dto.ScheduleSettings
	if err := json.Unmarshal(w2.Body.Bytes(), &got); err != nil {
		t.Fatalf("unmarshal: %v (%s)", err, w2.Body.String())
	}
	if !got.ScheduledStartEnabled || len(got.Rules) != 1 {
		t.Fatalf("schedule round-trip mismatch: %+v", got)
	}
	r := got.Rules[0]
	if r.StartHour != 9 || r.EndHour != 18 || r.LimitKBps != 500 || len(r.Days) != 5 {
		t.Fatalf("rule round-trip mismatch: %+v", r)
	}
}

func TestSchedulePutValidationRejectsBadHour(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	put := `{"scheduled_start_enabled":false,"rules":[{"start_hour":99,"end_hour":18,"limit_kbps":500}]}`
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPut, "/api/v1/settings/schedule", strings.NewReader(put))
	c.Request.Header.Set("Content-Type", "application/json")
	PutSchedule(c)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400 (%s)", w.Code, w.Body.String())
	}
}

func TestCategoriesGetPutRoundTrip(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())

	put := `{"enabled":true,"rules":[{"name":"Video","extensions":[".mp4",".mkv"],"dest_dir":"Videos"}]}`
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPut, "/api/v1/settings/categories", strings.NewReader(put))
	c.Request.Header.Set("Content-Type", "application/json")
	PutCategories(c)
	if w.Code != http.StatusOK {
		t.Fatalf("PUT status = %d, want 200 (%s)", w.Code, w.Body.String())
	}

	w2 := httptest.NewRecorder()
	c2, _ := gin.CreateTestContext(w2)
	c2.Request = httptest.NewRequest(http.MethodGet, "/api/v1/settings/categories", nil)
	GetCategories(c2)
	if w2.Code != http.StatusOK {
		t.Fatalf("GET status = %d, want 200 (%s)", w2.Code, w2.Body.String())
	}
	var got dto.CategorySettings
	if err := json.Unmarshal(w2.Body.Bytes(), &got); err != nil {
		t.Fatalf("unmarshal: %v (%s)", err, w2.Body.String())
	}
	if !got.Enabled || len(got.Rules) != 1 || got.Rules[0].Name != "Video" || len(got.Rules[0].Extensions) != 2 {
		t.Fatalf("categories round-trip mismatch: %+v", got)
	}
}

func TestCategoriesPutValidationRejectsEmptyName(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	put := `{"enabled":true,"rules":[{"name":"","extensions":[".mp4"],"dest_dir":"X"}]}`
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPut, "/api/v1/settings/categories", strings.NewReader(put))
	c.Request.Header.Set("Content-Type", "application/json")
	PutCategories(c)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400 (%s)", w.Code, w.Body.String())
	}
}
