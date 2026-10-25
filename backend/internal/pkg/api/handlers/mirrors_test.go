package handlers

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"

	"github.com/tiredbooy/Rum/backend/internal/pkg/api/dto"
)

func TestCreateDownloadStoresAndReturnsMirrors(t *testing.T) {
	setupManager(t)
	srv := newBatchFileServer(t)
	defer srv.Close()

	id := createOne(t, srv.URL, `,"mirrors":["https://m1.example/a.bin"," https://m2.example/a.bin ","https://m1.example/a.bin"]`)
	job, _ := GlobalManager.GetJob(id)
	if got := strings.Join(job.GetMirrors(), ","); got != "https://m1.example/a.bin,https://m2.example/a.bin" {
		t.Fatalf("mirrors = %q", got)
	}
	w := doJSON(t, GetDownloadStatus, http.MethodGet, "/api/v1/downloads/"+id, id, "")
	var resp dto.DownloadResponse
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil || len(resp.Mirrors) != 2 {
		t.Fatalf("response lacks mirrors: %v %s", err, w.Body.String())
	}
}

func TestCreateDownloadRejectsBadMirrors(t *testing.T) {
	setupManager(t)
	srv := newBatchFileServer(t)
	defer srv.Close()

	for name, extra := range map[string]string{
		"scheme":   `,"mirrors":["ftp://m.example/a.bin"]`,
		"too many": `,"mirrors":["http://1.example/a","http://2.example/a","http://3.example/a","http://4.example/a","http://5.example/a","http://6.example/a"]`,
	} {
		body := `{"urls":["` + srv.URL + `/a.bin"]` + extra + `}`
		w := doJSON(t, CreateDownload, http.MethodPost, "/api/v1/downloads", "", body)
		if w.Code != http.StatusBadRequest {
			t.Errorf("%s: status = %d (%s), want 400", name, w.Code, w.Body.String())
		}
	}
	body := `{"urls":["` + srv.URL + `/a.bin","` + srv.URL + `/b.bin"],"mirrors":["http://m.example/a"]}`
	if w := doJSON(t, CreateDownload, http.MethodPost, "/api/v1/downloads", "", body); w.Code != http.StatusBadRequest {
		t.Errorf("mirrors with two urls: status = %d, want 400", w.Code)
	}
}

func TestSetDownloadMirrors(t *testing.T) {
	setupManager(t)
	srv := newBatchFileServer(t)
	defer srv.Close()
	id := createOne(t, srv.URL, ``)

	w := doJSON(t, SetDownloadMirrors, http.MethodPut, "/api/v1/downloads/"+id+"/mirrors", id, `{"mirrors":["https://m.example/a.bin"]}`)
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d (%s)", w.Code, w.Body.String())
	}
	if job, _ := GlobalManager.GetJob(id); len(job.GetMirrors()) != 1 {
		t.Fatalf("mirrors not stored: %v", job.GetMirrors())
	}
	if w := doJSON(t, SetDownloadMirrors, http.MethodPut, "/x", id, `{"mirrors":["file:///etc/passwd"]}`); w.Code != http.StatusBadRequest {
		t.Fatalf("bad mirror status = %d", w.Code)
	}
	if w := doJSON(t, SetDownloadMirrors, http.MethodPut, "/x", "missing", `{"mirrors":[]}`); w.Code != http.StatusNotFound {
		t.Fatalf("unknown job status = %d", w.Code)
	}
}
