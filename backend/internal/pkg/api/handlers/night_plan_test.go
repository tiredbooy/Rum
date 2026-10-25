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

func nightReq(t *testing.T, method, body string, h gin.HandlerFunc) *httptest.ResponseRecorder {
	t.Helper()
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(method, "/api/v1/settings/night-plan", strings.NewReader(body))
	c.Request.Header.Set("Content-Type", "application/json")
	h(c)
	return w
}

func TestNightPlanDefaultsWhenNeverConfigured(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	w := nightReq(t, http.MethodGet, "", GetNightPlan)
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d (%s)", w.Code, w.Body.String())
	}
	var got dto.NightPlanResponse
	if err := json.Unmarshal(w.Body.Bytes(), &got); err != nil {
		t.Fatal(err)
	}
	if got.Enabled || got.Start != "02:00" || got.End != "07:00" || !got.StartAtOpen || !got.PauseAtClose {
		t.Fatalf("defaults = %+v", got)
	}
}

func TestNightPlanPutGetRoundTrip(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	put := `{"enabled":true,"start":"23:30","end":"06:15","start_at_open":true,"pause_at_close":false,"cap_gb":12.5}`
	if w := nightReq(t, http.MethodPut, put, PutNightPlan); w.Code != http.StatusOK {
		t.Fatalf("PUT status = %d (%s)", w.Code, w.Body.String())
	}
	w := nightReq(t, http.MethodGet, "", GetNightPlan)
	var got dto.NightPlanResponse
	if err := json.Unmarshal(w.Body.Bytes(), &got); err != nil {
		t.Fatal(err)
	}
	if !got.Enabled || got.Start != "23:30" || got.End != "06:15" || got.PauseAtClose || got.CapGB != 12.5 {
		t.Fatalf("round-trip = %+v", got)
	}
}

func TestNightPlanPutRejectsInvalid(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	for name, body := range map[string]string{
		"bad time":     `{"enabled":true,"start":"25:00","end":"07:00"}`,
		"empty window": `{"enabled":true,"start":"02:00","end":"02:00"}`,
		"negative cap": `{"enabled":true,"start":"02:00","end":"07:00","cap_gb":-1}`,
		"not json":     `nope`,
	} {
		if w := nightReq(t, http.MethodPut, body, PutNightPlan); w.Code != http.StatusBadRequest {
			t.Errorf("%s: status = %d, want 400 (%s)", name, w.Code, w.Body.String())
		}
	}
	// A rejected PUT must not have changed what is stored.
	w := nightReq(t, http.MethodGet, "", GetNightPlan)
	var got dto.NightPlanResponse
	_ = json.Unmarshal(w.Body.Bytes(), &got)
	if got.Enabled {
		t.Fatalf("invalid PUT leaked into settings: %+v", got)
	}
}

func TestNightQueueRequestValidate(t *testing.T) {
	if (&dto.NightQueueRequest{IDs: []string{"a"}, Enabled: true}).Validate() != nil {
		t.Fatal("valid request rejected")
	}
	if (&dto.NightQueueRequest{}).Validate() == nil {
		t.Fatal("empty ids accepted")
	}
	if (&dto.NightQueueRequest{IDs: []string{""}}).Validate() == nil {
		t.Fatal("blank id accepted")
	}
}
