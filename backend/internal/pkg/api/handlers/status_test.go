package handlers

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
)

type fakeStatus struct {
	online bool
	at     time.Time
}

func (f fakeStatus) Status() (bool, time.Time) { return f.online, f.at }

func getStatus(t *testing.T) map[string]any {
	t.Helper()
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.GET("/status", GetStatus)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/status", nil))
	if w.Code != http.StatusOK {
		t.Fatalf("status code = %d", w.Code)
	}
	var got map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &got); err != nil {
		t.Fatal(err)
	}
	return got
}

func TestGetStatusReportsConnectivity(t *testing.T) {
	old := GlobalResumer
	t.Cleanup(func() { GlobalResumer = old })

	GlobalResumer = nil
	got := getStatus(t)
	if got["online"] != true || got["last_probe_at"] != nil {
		t.Fatalf("no controller: %v; want online, no probe time", got)
	}

	at := time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC)
	GlobalResumer = fakeStatus{online: false, at: at}
	got = getStatus(t)
	if got["online"] != false || got["last_probe_at"] != "2026-01-02T03:04:05Z" {
		t.Fatalf("offline: %v", got)
	}
}
