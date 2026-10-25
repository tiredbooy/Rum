package updater

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func TestSourceURLsOrderAndNormalising(t *testing.T) {
	got := sourceURLs("https://api.github.com/x", []string{"https://ghproxy.net", " https://m2.example/ ", "", "ftp://bad", "javascript:alert(1)"})
	want := []string{
		"https://api.github.com/x",
		"https://ghproxy.net/https://api.github.com/x",
		"https://m2.example/https://api.github.com/x",
	}
	if strings.Join(got, "|") != strings.Join(want, "|") {
		t.Errorf("got %v\nwant %v", got, want)
	}
}

func releaseServer(t *testing.T, hits *int32) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if hits != nil {
			atomic.AddInt32(hits, 1)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"tag_name":"v2.0.0","assets":[]}`))
	}))
	t.Cleanup(srv.Close)
	return srv
}

func TestLatestParsesPrimary(t *testing.T) {
	srv := releaseServer(t, nil)
	c := &Checker{Client: srv.Client(), APIURL: srv.URL + "/releases/latest"}
	rel, err := c.Latest(context.Background())
	if err != nil || rel.Tag != "v2.0.0" {
		t.Fatalf("rel=%+v err=%v", rel, err)
	}
}

func TestLatestFallsBackToMirror(t *testing.T) {
	dead := httptest.NewServer(http.NotFoundHandler())
	deadURL := dead.URL
	dead.Close() // connection refused

	var mirrorHits int32
	mirror := releaseServer(t, &mirrorHits)
	c := &Checker{
		Client:  mirror.Client(),
		APIURL:  deadURL + "/releases/latest",
		Mirrors: []string{mirror.URL},
	}
	rel, err := c.Latest(context.Background())
	if err != nil || rel.Tag != "v2.0.0" {
		t.Fatalf("rel=%+v err=%v", rel, err)
	}
	if mirrorHits != 1 {
		t.Errorf("mirror hits = %d", mirrorHits)
	}
}

func TestLatestSkipsBadPayloadAndStatus(t *testing.T) {
	bad := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte("<html>captive portal</html>"))
	}))
	defer bad.Close()
	rl := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		http.Error(w, "rate limited", http.StatusForbidden)
	}))
	defer rl.Close()
	good := releaseServer(t, nil)
	c := &Checker{Client: good.Client(), APIURL: bad.URL + "/r", Mirrors: []string{rl.URL, good.URL}}
	if rel, err := c.Latest(context.Background()); err != nil || rel.Tag != "v2.0.0" {
		t.Fatalf("rel=%+v err=%v", rel, err)
	}
}

func TestLatestAllSourcesFail(t *testing.T) {
	dead := httptest.NewServer(http.NotFoundHandler())
	u := dead.URL
	dead.Close()
	c := &Checker{Client: http.DefaultClient, APIURL: u, Mirrors: []string{u}}
	if _, err := c.Latest(context.Background()); !errors.Is(err, ErrUnreachable) {
		t.Fatalf("err = %v, want ErrUnreachable", err)
	}
}

func TestLatestShortTimeoutPerSource(t *testing.T) {
	slow := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		select {
		case <-r.Context().Done():
		case <-time.After(3 * time.Second):
		}
	}))
	defer slow.Close()
	good := releaseServer(t, nil)
	c := &Checker{Client: good.Client(), APIURL: slow.URL, Mirrors: []string{good.URL}, Timeout: 150 * time.Millisecond}
	start := time.Now()
	rel, err := c.Latest(context.Background())
	if err != nil || rel.Tag != "v2.0.0" {
		t.Fatalf("rel=%+v err=%v", rel, err)
	}
	if time.Since(start) > 2*time.Second {
		t.Errorf("took %v; the slow source was not cut off", time.Since(start))
	}
}

func TestLatestRejectsOversizedBody(t *testing.T) {
	big := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"tag_name":"v9.0.0","pad":"` + strings.Repeat("a", maxAPIBytes) + `"}`))
	}))
	defer big.Close()
	c := &Checker{Client: big.Client(), APIURL: big.URL}
	if _, err := c.Latest(context.Background()); err == nil {
		t.Fatal("oversized body must be rejected")
	}
}

// assetServer serves a release payload for APIURL plus the asset files, and is
// used as both primary (with absolute URLs rewritten by a transport) and mirror.
func TestDownloadVerified(t *testing.T) {
	payload := []byte("rum-binary-bytes")
	good := sum(payload)
	mk := func(sums string, body []byte) *Checker {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			switch {
			case strings.HasSuffix(r.URL.Path, "/SHA256SUMS"):
				_, _ = w.Write([]byte(sums))
			case strings.HasSuffix(r.URL.Path, "/Rum-2.0.0-linux-amd64.AppImage"):
				_, _ = w.Write(body)
			default:
				http.NotFound(w, r)
			}
		}))
		t.Cleanup(srv.Close)
		// Every request goes to srv regardless of host: models the real hosts.
		cl := &http.Client{Transport: rewriteTo(srv)}
		return &Checker{Client: cl, APIURL: "https://api.github.com/x"}
	}
	rel, _ := ParseRelease([]byte(`{"tag_name":"v2.0.0","assets":[
		{"name":"Rum-2.0.0-linux-amd64.AppImage"},{"name":"SHA256SUMS"}]}`))

	t.Run("ok", func(t *testing.T) {
		dir := t.TempDir()
		p, err := mk(good+"  Rum-2.0.0-linux-amd64.AppImage\n", payload).DownloadVerified(context.Background(), rel, "linux", "amd64", dir)
		if err != nil {
			t.Fatal(err)
		}
		if filepath.Dir(p) != dir || filepath.Base(p) != "Rum-2.0.0-linux-amd64.AppImage" {
			t.Errorf("path = %s", p)
		}
		if b, _ := os.ReadFile(p); string(b) != string(payload) {
			t.Error("content differs")
		}
	})
	t.Run("mismatch deletes the file", func(t *testing.T) {
		dir := t.TempDir()
		_, err := mk(good+"  Rum-2.0.0-linux-amd64.AppImage\n", []byte("tampered")).DownloadVerified(context.Background(), rel, "linux", "amd64", dir)
		if !errors.Is(err, ErrChecksumMismatch) {
			t.Fatalf("err = %v", err)
		}
		if entries, _ := os.ReadDir(dir); len(entries) != 0 {
			t.Errorf("left behind: %v", entries)
		}
	})
	t.Run("no checksum entry refuses", func(t *testing.T) {
		dir := t.TempDir()
		_, err := mk("", payload).DownloadVerified(context.Background(), rel, "linux", "amd64", dir)
		if !errors.Is(err, ErrNoChecksum) {
			t.Fatalf("err = %v", err)
		}
		if entries, _ := os.ReadDir(dir); len(entries) != 0 {
			t.Errorf("left behind: %v", entries)
		}
	})
	t.Run("no asset for platform", func(t *testing.T) {
		_, err := mk("", payload).DownloadVerified(context.Background(), rel, "windows", "amd64", t.TempDir())
		if !errors.Is(err, ErrNoAsset) {
			t.Fatalf("err = %v", err)
		}
	})
}

type rewriteTransport struct{ target *httptest.Server }

func rewriteTo(s *httptest.Server) http.RoundTripper { return rewriteTransport{s} }

func (t rewriteTransport) RoundTrip(r *http.Request) (*http.Response, error) {
	r2 := r.Clone(r.Context())
	r2.URL.Scheme = "http"
	r2.URL.Host = strings.TrimPrefix(t.target.URL, "http://")
	return http.DefaultTransport.RoundTrip(r2)
}
