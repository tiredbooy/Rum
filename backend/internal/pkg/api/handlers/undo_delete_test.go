package handlers

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
)

func doQuery(t *testing.T, h gin.HandlerFunc, method, url, id, body string) *httptest.ResponseRecorder {
	t.Helper()
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(method, url, strings.NewReader(body))
	c.Request.Header.Set("Content-Type", "application/json")
	if id != "" {
		c.Params = gin.Params{{Key: "id", Value: id}}
	}
	h(c)
	return w
}

func TestDeleteThenRestoreDownload(t *testing.T) {
	setupManager(t)
	srv := newBatchFileServer(t)
	defer srv.Close()
	id := createOne(t, srv.URL, `,"start":false`)

	w := doQuery(t, DeleteDownload, http.MethodDelete, "/api/v1/downloads/"+id+"?delete_file=true", id, "")
	if w.Code != http.StatusOK {
		t.Fatalf("delete status = %d (%s)", w.Code, w.Body.String())
	}
	var resp struct {
		IDs             []string `json:"ids"`
		PendingDeleteAt string   `json:"pending_delete_at"`
		UndoSeconds     int      `json:"undo_seconds"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil || len(resp.IDs) != 1 || resp.PendingDeleteAt == "" || resp.UndoSeconds <= 0 {
		t.Fatalf("bad delete body: %v %s", err, w.Body.String())
	}
	if _, ok := GlobalManager.GetJob(id); ok {
		t.Fatal("job still visible after delete")
	}

	w = doQuery(t, RestoreDownload, http.MethodPost, "/api/v1/downloads/"+id+"/restore", id, "")
	if w.Code != http.StatusOK {
		t.Fatalf("restore status = %d (%s)", w.Code, w.Body.String())
	}
	if _, ok := GlobalManager.GetJob(id); !ok {
		t.Fatal("job not restored")
	}
	w = doQuery(t, RestoreDownload, http.MethodPost, "/api/v1/downloads/"+id+"/restore", id, "")
	if w.Code != http.StatusNotFound {
		t.Fatalf("restoring a visible job = %d, want 404", w.Code)
	}
}

func TestBulkDeleteAndRestore(t *testing.T) {
	setupManager(t)
	srv := newBatchFileServer(t)
	defer srv.Close()
	id := createOne(t, srv.URL, `,"start":false`)

	w := doQuery(t, DeleteDownloads, http.MethodDelete, "/api/v1/downloads?status=all", "", "")
	if w.Code != http.StatusOK {
		t.Fatalf("bulk delete = %d (%s)", w.Code, w.Body.String())
	}
	var resp struct {
		IDs []string `json:"ids"`
	}
	_ = json.Unmarshal(w.Body.Bytes(), &resp)
	found := false
	for _, x := range resp.IDs {
		found = found || x == id
	}
	if !found {
		t.Fatalf("ids = %v, want to contain %s", resp.IDs, id)
	}
	body, _ := json.Marshal(map[string]any{"ids": resp.IDs})
	w = doQuery(t, RestoreDownloads, http.MethodPost, "/api/v1/downloads/restore", "", string(body))
	if w.Code != http.StatusOK || !strings.Contains(w.Body.String(), `"restored":`) {
		t.Fatalf("bulk restore = %d %s", w.Code, w.Body.String())
	}
	if _, ok := GlobalManager.GetJob(id); !ok {
		t.Fatal("job not restored")
	}
}
