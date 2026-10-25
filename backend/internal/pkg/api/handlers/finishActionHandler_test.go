package handlers

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/tiredbooy/Rum/backend/internal/pkg/download"
)

func TestFinishActionEndpointsIdleAndCancel(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	InitAPI(&download.Options{Parallel: 1, Connections: 1, Silent: true})
	defer GlobalManager.Shutdown()

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "/api/v1/finish-action", nil)
	GetFinishAction(c)
	var got map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &got); err != nil || w.Code != 200 {
		t.Fatalf("get: %d %s %v", w.Code, w.Body.String(), err)
	}
	if v, ok := got["pending"]; !ok || v != nil {
		t.Fatalf("pending = %v, want null", v)
	}

	w = httptest.NewRecorder()
	c, _ = gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPost, "/api/v1/finish-action/cancel", nil)
	CancelFinishAction(c)
	if w.Code != 200 || w.Body.String() != `{"cancelled":false}` {
		t.Fatalf("cancel: %d %s", w.Code, w.Body.String())
	}
}
