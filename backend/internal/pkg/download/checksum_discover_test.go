package download

import (
	"context"
	"crypto/md5"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestParseChecksumFile(t *testing.T) {
	payload := []byte("hello rum")
	h256, h5 := sha256Hex(payload), md5Hex(payload)

	cases := []struct {
		name     string
		body     string
		file     string
		wantAlgo string
		wantHex  string
		ok       bool
	}{
		{"bare sha256", h256 + "\n", "a.zip", "sha256", h256, true},
		{"bare md5", h5, "a.zip", "md5", h5, true},
		{"two spaces name", h256 + "  a.zip\n", "a.zip", "sha256", h256, true},
		{"binary star", h256 + " *a.zip\n", "a.zip", "sha256", h256, true},
		{"dot slash", h256 + "  ./a.zip\n", "a.zip", "sha256", h256, true},
		{"uppercase hex", strings.ToUpper(h256) + "  a.zip", "a.zip", "sha256", h256, true},
		{"picks matching line", sha256Hex([]byte("x")) + "  b.zip\n" + h256 + "  a.zip\n", "a.zip", "sha256", h256, true},
		{"crlf", h256 + "  a.zip\r\n", "a.zip", "sha256", h256, true},
		{"bsd style", "SHA256 (a.zip) = " + h256 + "\n", "a.zip", "sha256", h256, true},
		{"md5 bsd", "MD5 (a.zip) = " + h5 + "\n", "a.zip", "md5", h5, true},
		{"other file only", h256 + "  b.zip\n", "a.zip", "", "", false},
		{"nameless but several lines", h256 + "\n" + h256 + "\n", "a.zip", "", "", false},
		{"html", "<html><body>404</body></html>", "a.zip", "", "", false},
		{"wrong length", "abcdef  a.zip", "a.zip", "", "", false},
		{"empty", "", "a.zip", "", "", false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			d, ok := ParseChecksumFile([]byte(tc.body), tc.file)
			if ok != tc.ok {
				t.Fatalf("ok = %v, want %v (%+v)", ok, tc.ok, d)
			}
			if ok && (d.Algo != tc.wantAlgo || d.Hex != tc.wantHex) {
				t.Fatalf("got %s %s, want %s %s", d.Algo, d.Hex, tc.wantAlgo, tc.wantHex)
			}
		})
	}
}

func TestDigestFromHeaders(t *testing.T) {
	payload := []byte("hello rum")
	raw256 := sha256sumBytes(payload)
	raw5 := md5sumBytes(payload)
	b256 := base64.StdEncoding.EncodeToString(raw256)
	b5 := base64.StdEncoding.EncodeToString(raw5)

	cases := []struct {
		name    string
		headers map[string]string
		algo    string
		hex     string
		ok      bool
	}{
		{"digest sha-256", map[string]string{"Digest": "sha-256=" + b256}, "sha256", hex.EncodeToString(raw256), true},
		{"digest SHA-256 upper", map[string]string{"Digest": "SHA-256=" + b256}, "sha256", hex.EncodeToString(raw256), true},
		{"digest list picks sha256", map[string]string{"Digest": "md5=" + b5 + ", sha-256=" + b256}, "sha256", hex.EncodeToString(raw256), true},
		{"content-digest structured", map[string]string{"Content-Digest": "sha-256=:" + b256 + ":"}, "sha256", hex.EncodeToString(raw256), true},
		{"content-md5", map[string]string{"Content-MD5": b5}, "md5", hex.EncodeToString(raw5), true},
		{"x-goog-hash", map[string]string{"X-Goog-Hash": "crc32c=AAAAAA==, md5=" + b5}, "md5", hex.EncodeToString(raw5), true},
		{"unsupported only", map[string]string{"Digest": "crc32c=AAAAAA=="}, "", "", false},
		{"garbage base64", map[string]string{"Content-MD5": "!!!"}, "", "", false},
		{"wrong length", map[string]string{"Content-MD5": base64.StdEncoding.EncodeToString([]byte("short"))}, "", "", false},
		{"none", nil, "", "", false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			h := http.Header{}
			for k, v := range tc.headers {
				h.Set(k, v)
			}
			d, ok := DigestFromHeaders(h)
			if ok != tc.ok {
				t.Fatalf("ok = %v, want %v", ok, tc.ok)
			}
			if ok && (d.Algo != tc.algo || d.Hex != tc.hex || d.Source != ChecksumSourceHeader) {
				t.Fatalf("got %+v", d)
			}
		})
	}
}

// fileServer serves a 'file' at /dir/a.zip plus whatever extras the test adds,
// recording every path it was asked for.
type fileServer struct {
	*httptest.Server
	mu   sync.Mutex
	seen []string
}

func newFileServer(t *testing.T, extra map[string]string, hdr http.Header) *fileServer {
	t.Helper()
	fs := &fileServer{}
	fs.Server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fs.mu.Lock()
		fs.seen = append(fs.seen, r.Method+" "+r.URL.Path)
		fs.mu.Unlock()
		if r.URL.Path == "/dir/a.zip" {
			for k, v := range hdr {
				w.Header()[k] = v
			}
			w.Header().Set("Content-Type", "application/zip")
			w.Write([]byte("hello rum"))
			return
		}
		if body, ok := extra[r.URL.Path]; ok {
			w.Header().Set("Content-Type", "text/plain")
			w.Write([]byte(body))
			return
		}
		http.NotFound(w, r)
	}))
	t.Cleanup(fs.Close)
	return fs
}

func (fs *fileServer) saw(s string) bool {
	fs.mu.Lock()
	defer fs.mu.Unlock()
	for _, x := range fs.seen {
		if x == s {
			return true
		}
	}
	return false
}

func discoverCtx() (context.Context, context.CancelFunc) {
	return context.WithTimeout(context.Background(), 10*time.Second)
}

func TestDiscoverChecksumSidecars(t *testing.T) {
	h256 := sha256Hex([]byte("hello rum"))
	h5 := md5Hex([]byte("hello rum"))

	cases := []struct {
		name  string
		extra map[string]string
		algo  string
		hex   string
	}{
		{"sha256 sidecar", map[string]string{"/dir/a.zip.sha256": h256 + "  a.zip\n"}, "sha256", h256},
		{"sha256sum sidecar", map[string]string{"/dir/a.zip.sha256sum": h256 + "\n"}, "sha256", h256},
		{"md5 sidecar", map[string]string{"/dir/a.zip.md5": h5 + " *a.zip"}, "md5", h5},
		{"SHA256SUMS", map[string]string{"/dir/SHA256SUMS": sha256Hex([]byte("z")) + "  z.zip\n" + h256 + "  a.zip\n"}, "sha256", h256},
		{"sha256sums.txt", map[string]string{"/dir/sha256sums.txt": h256 + " *a.zip\n"}, "sha256", h256},
		{"sha256 beats md5", map[string]string{"/dir/a.zip.md5": h5, "/dir/a.zip.sha256": h256}, "sha256", h256},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			srv := newFileServer(t, tc.extra, nil)
			ctx, cancel := discoverCtx()
			defer cancel()
			d, ok := DiscoverChecksum(ctx, NewDownloader("", ""), srv.URL+"/dir/a.zip", "a.zip")
			if !ok {
				t.Fatalf("nothing discovered; requests: %v", srv.seen)
			}
			if d.Algo != tc.algo || d.Hex != tc.hex {
				t.Fatalf("got %+v", d)
			}
		})
	}
}

func TestDiscoverChecksumNothingAvailable(t *testing.T) {
	srv := newFileServer(t, nil, nil)
	ctx, cancel := discoverCtx()
	defer cancel()
	if d, ok := DiscoverChecksum(ctx, NewDownloader("", ""), srv.URL+"/dir/a.zip", "a.zip"); ok {
		t.Fatalf("unexpected digest %+v", d)
	}
}

func TestDiscoverChecksumUsesHeaders(t *testing.T) {
	raw := md5sumBytes([]byte("hello rum"))
	hdr := http.Header{"Content-Md5": {base64.StdEncoding.EncodeToString(raw)}}
	srv := newFileServer(t, nil, hdr)
	ctx, cancel := discoverCtx()
	defer cancel()
	d, ok := DiscoverChecksum(ctx, NewDownloader("", ""), srv.URL+"/dir/a.zip", "a.zip")
	if !ok || d.Algo != "md5" || d.Source != ChecksumSourceHeader {
		t.Fatalf("got %+v ok=%v", d, ok)
	}
}

func TestDiscoverChecksumRejectsOtherHostRedirect(t *testing.T) {
	h256 := sha256Hex([]byte("hello rum"))
	other := newFileServer(t, map[string]string{"/dir/a.zip.sha256": h256 + "  a.zip"}, nil)
	origin := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Every sidecar redirects to another host (a different port is a different host).
		http.Redirect(w, r, other.URL+r.URL.Path, http.StatusFound)
	}))
	defer origin.Close()

	ctx, cancel := discoverCtx()
	defer cancel()
	if d, ok := DiscoverChecksum(ctx, NewDownloader("", ""), origin.URL+"/dir/a.zip", "a.zip"); ok {
		t.Fatalf("accepted a checksum from another host: %+v", d)
	}
	if other.saw("GET /dir/a.zip.sha256") {
		t.Fatal("followed the redirect to the other host")
	}
}

func TestDiscoverChecksumRejectsLargeAndHTML(t *testing.T) {
	h256 := sha256Hex([]byte("hello rum"))
	big := h256 + "  a.zip\n" + strings.Repeat("x", (1<<20)+10)
	srv := newFileServer(t, map[string]string{
		"/dir/a.zip.sha256":    big,
		"/dir/a.zip.sha256sum": "<html>" + h256 + "  a.zip</html>",
	}, nil)
	ctx, cancel := discoverCtx()
	defer cancel()
	if d, ok := DiscoverChecksum(ctx, NewDownloader("", ""), srv.URL+"/dir/a.zip", "a.zip"); ok {
		t.Fatalf("accepted unusable sidecar: %+v", d)
	}
}

func TestDiscoverChecksumKeepsQueryOut(t *testing.T) {
	h256 := sha256Hex([]byte("hello rum"))
	srv := newFileServer(t, map[string]string{"/dir/a.zip.sha256": h256}, nil)
	ctx, cancel := discoverCtx()
	defer cancel()
	d, ok := DiscoverChecksum(ctx, NewDownloader("", ""), srv.URL+"/dir/a.zip?token=secret", "a.zip")
	if !ok || d.Hex != h256 {
		t.Fatalf("got %+v ok=%v", d, ok)
	}
}

func sha256sumBytes(b []byte) []byte { s := sha256.Sum256(b); return s[:] }
func md5sumBytes(b []byte) []byte    { s := md5.Sum(b); return s[:] }
