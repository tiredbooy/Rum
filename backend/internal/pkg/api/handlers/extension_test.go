package handlers

import (
	"bytes"
	"encoding/json"
	"log"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
)

func extAddBody(url, extra string) string {
	return `{"url":"` + url + `","auto_start":false` + extra + `}`
}

func TestExtAddCreatesJobWithHeadersAndCookie(t *testing.T) {
	setupManager(t)
	srv := newBatchFileServer(t)
	defer srv.Close()

	var logs bytes.Buffer
	log.SetOutput(&logs)
	defer log.SetOutput(os.Stderr)

	w := doJSON(t, ExtAdd, http.MethodPost, "/ext/add", "", extAddBody(srv.URL+"/dl?id=1",
		`,"filename":"Report 2024.pdf","referer":"https://site.example/page","user_agent":"Browser/1","cookie":"sid=SECRETVALUE; a=b"`))
	if w.Code != http.StatusCreated {
		t.Fatalf("status %d: %s", w.Code, w.Body.String())
	}
	var resp struct {
		ID        string `json:"id"`
		Duplicate bool   `json:"duplicate"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil || resp.ID == "" {
		t.Fatalf("bad body %s (%v)", w.Body.String(), err)
	}
	job, ok := GlobalManager.GetJob(resp.ID)
	if !ok {
		t.Fatal("job missing")
	}
	if job.GetReferer() != "https://site.example/page" || job.GetUserAgent() != "Browser/1" ||
		job.GetCookie() != "sid=SECRETVALUE; a=b" || job.FileName != "Report 2024.pdf" {
		t.Fatalf("fields dropped: ref=%q ua=%q cookie=%q name=%q", job.GetReferer(), job.GetUserAgent(), job.GetCookie(), job.FileName)
	}

	// Never returned by any API response, never logged.
	if strings.Contains(w.Body.String(), "SECRETVALUE") {
		t.Fatalf("add response leaks cookie: %s", w.Body.String())
	}
	st := doJSON(t, GetDownloadStatus, http.MethodGet, "/api/v1/downloads/"+resp.ID, resp.ID, "")
	if strings.Contains(st.Body.String(), "SECRETVALUE") {
		t.Fatalf("status leaks cookie: %s", st.Body.String())
	}
	list := doJSON(t, GetAllJobs, http.MethodGet, "/api/v1/downloads", "", "")
	if strings.Contains(list.Body.String(), "SECRETVALUE") {
		t.Fatalf("list leaks cookie: %s", list.Body.String())
	}
	if strings.Contains(logs.String(), "SECRETVALUE") {
		t.Fatalf("cookie written to logs: %s", logs.String())
	}
}

func TestExtAddRejectsBadInput(t *testing.T) {
	setupManager(t)
	srv := newBatchFileServer(t)
	defer srv.Close()

	cases := map[string]string{
		"blob":        extAddBody("blob:https://x.example/uuid", ""),
		"data":        extAddBody("data:text/plain;base64,AAAA", ""),
		"ftp":         extAddBody("ftp://x.example/f", ""),
		"empty":       extAddBody("", ""),
		"cookie crlf": extAddBody(srv.URL+"/a", `,"cookie":"a=b\r\nX-Evil: 1"`),
		"cookie huge": extAddBody(srv.URL+"/a", `,"cookie":"`+strings.Repeat("a", 20000)+`"`),
		"not json":    `nope`,
	}
	for name, body := range cases {
		if w := doJSON(t, ExtAdd, http.MethodPost, "/ext/add", "", body); w.Code != http.StatusBadRequest {
			t.Errorf("%s: status %d, want 400 (%s)", name, w.Code, w.Body.String())
		}
	}
}

func TestExtAddDuplicateIsAccepted(t *testing.T) {
	setupManager(t)
	srv := newBatchFileServer(t)
	defer srv.Close()
	u := srv.URL + "/same.bin"

	first := doJSON(t, ExtAdd, http.MethodPost, "/ext/add", "", extAddBody(u, ""))
	if first.Code != http.StatusCreated {
		t.Fatalf("first: %d %s", first.Code, first.Body.String())
	}
	second := doJSON(t, ExtAdd, http.MethodPost, "/ext/add", "", extAddBody(u, ""))
	if second.Code != http.StatusOK || !strings.Contains(second.Body.String(), `"duplicate":true`) {
		t.Fatalf("duplicate: %d %s", second.Code, second.Body.String())
	}
}

func TestExtPing(t *testing.T) {
	w := httptest.NewRecorder()
	c, _ := newTestCtx(w)
	ExtPing(c)
	if w.Code != 200 || !strings.Contains(w.Body.String(), `"app":"rum"`) {
		t.Fatalf("ping: %d %s", w.Code, w.Body.String())
	}
}

func TestExtensionTokenEndpoints(t *testing.T) {
	setupManager(t)
	get := func() string {
		w := doJSON(t, GetExtensionToken, http.MethodGet, "/api/v1/extension/token", "", "")
		var r struct{ Token string }
		_ = json.Unmarshal(w.Body.Bytes(), &r)
		if w.Code != 200 || !strings.HasPrefix(r.Token, "rum_") {
			t.Fatalf("get: %d %s", w.Code, w.Body.String())
		}
		return r.Token
	}
	a := get()
	if get() != a {
		t.Fatal("token not stable")
	}
	w := doJSON(t, RegenerateExtensionToken, http.MethodPost, "/api/v1/extension/token/regenerate", "", `{}`)
	var r struct{ Token string }
	_ = json.Unmarshal(w.Body.Bytes(), &r)
	if w.Code != 200 || r.Token == a || get() != r.Token {
		t.Fatalf("regenerate: %d %s", w.Code, w.Body.String())
	}
}

func TestRegenerateTokenRequiresJSONContentType(t *testing.T) {
	setupManager(t)
	w := httptest.NewRecorder()
	c, _ := newTestCtx(w)
	c.Request = httptest.NewRequest(http.MethodPost, "/api/v1/extension/token/regenerate", strings.NewReader("x"))
	c.Request.Header.Set("Content-Type", "text/plain")
	RegenerateExtensionToken(c)
	if w.Code != http.StatusUnsupportedMediaType {
		t.Fatalf("status %d, want 415", w.Code)
	}
}

func newTestCtx(w *httptest.ResponseRecorder) (*gin.Context, *gin.Engine) {
	c, e := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "/", nil)
	return c, e
}
