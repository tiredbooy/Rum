package download

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
)

// mirrorTestServer serves payload with Range support. After limit body bytes
// (limit >= 0) it "dies": the current response is cut short and every later
// request, HEAD included, gets its connection dropped.
type mirrorTestServer struct {
	*httptest.Server
	payload []byte
	limit   int64
	served  atomic.Int64
	mu      sync.Mutex
	ranges  []string
	cookies []string
}

func newMirrorTestServer(t *testing.T, payload []byte, limit int64) *mirrorTestServer {
	t.Helper()
	ms := &mirrorTestServer{payload: payload, limit: limit}
	ms.Server = httptest.NewServer(http.HandlerFunc(ms.handle))
	t.Cleanup(ms.Close)
	return ms
}

func (ms *mirrorTestServer) drop(w http.ResponseWriter) {
	if hj, ok := w.(http.Hijacker); ok {
		if conn, _, err := hj.Hijack(); err == nil {
			conn.Close()
		}
	}
}

func (ms *mirrorTestServer) handle(w http.ResponseWriter, r *http.Request) {
	ms.mu.Lock()
	if r.Method == http.MethodGet {
		ms.ranges = append(ms.ranges, r.Header.Get("Range"))
	}
	ms.cookies = append(ms.cookies, r.Header.Get("Cookie"))
	ms.mu.Unlock()

	if ms.limit >= 0 && ms.served.Load() >= ms.limit {
		ms.drop(w)
		return
	}
	w.Header().Set("Accept-Ranges", "bytes")
	start, end, status := 0, len(ms.payload)-1, http.StatusOK
	if rh := r.Header.Get("Range"); rh != "" {
		s, e, ok := parseTestRange(rh, len(ms.payload))
		if !ok {
			w.WriteHeader(http.StatusRequestedRangeNotSatisfiable)
			return
		}
		start, end, status = s, e, http.StatusPartialContent
		w.Header().Set("Content-Range", fmt.Sprintf("bytes %d-%d/%d", s, e, len(ms.payload)))
	}
	body := ms.payload[start : end+1]
	w.Header().Set("Content-Length", strconv.Itoa(len(body)))
	w.WriteHeader(status)
	if r.Method == http.MethodHead {
		return
	}
	if ms.limit >= 0 {
		ms.mu.Lock() // reserve atomically: segments request concurrently
		take := min(int64(len(body)), ms.limit-ms.served.Load())
		ms.served.Add(take)
		ms.mu.Unlock()
		if take < int64(len(body)) {
			w.Write(body[:take])
			w.(http.Flusher).Flush()
			ms.drop(w)
			return
		}
		w.Write(body)
		return
	}
	w.Write(body)
	ms.served.Add(int64(len(body)))
}

func (ms *mirrorTestServer) seenRanges() []string {
	ms.mu.Lock()
	defer ms.mu.Unlock()
	return append([]string(nil), ms.ranges...)
}

func mirrorJobOpt(t *testing.T, primary *mirrorTestServer, mirrors []string, conns int) (*Job, Options, string) {
	t.Helper()
	dir := t.TempDir()
	job := &Job{ID: "mirror", URL: primary.URL + "/file.bin", FileName: "file.bin",
		TotalSize: int64(len(primary.payload)), SupportRange: true, Status: StatusPending,
		Mirrors: mirrors}
	opt := Options{Out: dir, Connections: conns, Downloader: NewDownloader("test-agent", ""), MaxRetries: 0}
	return job, opt, dir
}

func readFileIfExists(p string) ([]byte, error) { return os.ReadFile(p) }

func noFullFetch(t *testing.T, ms *mirrorTestServer) {
	t.Helper()
	for _, r := range ms.seenRanges() {
		if r == "" || r == "bytes=0-" {
			t.Fatalf("mirror was asked for the whole file (Range %q): download restarted", r)
		}
	}
}

func TestSingleStreamFailsOverToMirror(t *testing.T) {
	payload := makePayload(600 * 1024)
	primary := newMirrorTestServer(t, payload, 200*1024)
	mirror := newMirrorTestServer(t, payload, -1)
	job, opt, dir := mirrorJobOpt(t, primary, []string{mirror.URL + "/copy.bin"}, 1)

	if err := DownloadSingleFile(context.Background(), opt, job, nil); err != nil {
		t.Fatalf("expected completion from mirror, got %v", err)
	}
	assertFile(t, filepath.Join(dir, "file.bin"), payload)
	noFullFetch(t, mirror)
	if got, want := job.GetActiveMirror(), mirror.Listener.Addr().String(); got != want {
		t.Fatalf("active mirror = %q, want %q", got, want)
	}
}

func TestMirrorWithOtherSizeIsRejected(t *testing.T) {
	payload := makePayload(600 * 1024)
	primary := newMirrorTestServer(t, payload, 200*1024)
	mirror := newMirrorTestServer(t, makePayload(600*1024+1), -1)
	job, opt, dir := mirrorJobOpt(t, primary, []string{mirror.URL + "/copy.bin"}, 1)

	if err := DownloadSingleFile(context.Background(), opt, job, nil); err == nil {
		t.Fatal("download must fail rather than continue from a different file")
	}
	if len(mirror.seenRanges()) != 0 {
		t.Fatalf("no data may be fetched from a size-mismatched mirror: %v", mirror.seenRanges())
	}
	if got := job.GetActiveMirror(); got != "" {
		t.Fatalf("rejected mirror must not become active, got %q", got)
	}
	if fi, err := filepath.Glob(filepath.Join(dir, "file.bin")); err != nil || len(fi) != 1 {
		t.Fatalf("partial data must be kept: %v %v", fi, err)
	}
}

func TestMirrorWithOtherBytesIsRejected(t *testing.T) {
	payload := makePayload(600 * 1024)
	other := bytes.Clone(payload)
	for i := 0; i < 64; i++ { // same size, different content inside the sampled head
		other[i] ^= 0xFF
	}
	primary := newMirrorTestServer(t, payload, 200*1024)
	mirror := newMirrorTestServer(t, other, -1)
	job, opt, dir := mirrorJobOpt(t, primary, []string{mirror.URL + "/copy.bin"}, 1)

	if err := DownloadSingleFile(context.Background(), opt, job, nil); err == nil {
		t.Fatal("same-size but different file must be rejected")
	}
	got, _ := readFileIfExists(filepath.Join(dir, "file.bin"))
	if len(got) > 0 && !bytes.Equal(got, payload[:len(got)]) {
		t.Fatal("bytes of the other file were spliced into the download")
	}
}

func TestSegmentedReassignsToMirror(t *testing.T) {
	payload := makePayload(5 * 1024 * 1024)
	primary := newMirrorTestServer(t, payload, 1536*1024)
	mirror := newMirrorTestServer(t, payload, -1)
	job, opt, dir := mirrorJobOpt(t, primary, []string{mirror.URL + "/copy.bin"}, 4)

	if err := DownloadSegmented(context.Background(), opt, job, nil); err != nil {
		t.Fatalf("expected completion from mirror, got %v", err)
	}
	assertFile(t, filepath.Join(dir, "file.bin"), payload)
	noFullFetch(t, mirror)
	if job.GetActiveMirror() == "" {
		t.Fatal("active mirror not recorded")
	}
	// Segments that were already done on the primary must not be refetched.
	if mirror.served.Load() >= int64(len(payload)) {
		t.Fatalf("mirror served %d bytes: finished work was refetched", mirror.served.Load())
	}
}

func TestSegmentedMismatchedMirrorKeepsFailure(t *testing.T) {
	payload := makePayload(5 * 1024 * 1024)
	other := bytes.Clone(payload)
	for i := 0; i < len(other); i += 4096 { // differ everywhere, whichever segments got data
		other[i] ^= 0xFF
	}
	primary := newMirrorTestServer(t, payload, 1536*1024)
	mirror := newMirrorTestServer(t, other, -1)
	job, opt, dir := mirrorJobOpt(t, primary, []string{mirror.URL + "/copy.bin"}, 4)

	if err := DownloadSegmented(context.Background(), opt, job, nil); err == nil {
		t.Fatal("must fail: the only mirror is a different file")
	}
	got, _ := readFileIfExists(filepath.Join(dir, "file.bin"))
	for i := range got { // every byte on disk is either primary data or an unfilled hole
		if got[i] != 0 && got[i] != payload[i] {
			t.Fatalf("foreign byte at %d", i)
		}
	}
}

func TestMirrorDoesNotReceiveCookie(t *testing.T) {
	payload := makePayload(600 * 1024)
	primary := newMirrorTestServer(t, payload, 200*1024)
	mirror := newMirrorTestServer(t, payload, -1)
	job, opt, _ := mirrorJobOpt(t, primary, []string{mirror.URL + "/copy.bin"}, 1)
	opt.Downloader.Headers["Cookie"] = "session=secret"

	if err := DownloadSingleFile(context.Background(), opt, job, nil); err != nil {
		t.Fatal(err)
	}
	for _, c := range mirror.cookies {
		if c != "" {
			t.Fatalf("cookie leaked to mirror host: %q", c)
		}
	}
	sawPrimary := false
	for _, c := range primary.cookies {
		sawPrimary = sawPrimary || c == "session=secret"
	}
	if !sawPrimary {
		t.Fatal("primary should still get the cookie")
	}
}

func TestNormalizeMirrors(t *testing.T) {
	primary := "https://a.example/f.zip"
	got, err := NormalizeMirrors(primary, []string{" https://b.example/f.zip ", "https://b.example/f.zip", primary, "http://c.example/f.zip"})
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"https://b.example/f.zip", "http://c.example/f.zip"}
	if strings.Join(got, ",") != strings.Join(want, ",") {
		t.Fatalf("got %v want %v", got, want)
	}
	for _, bad := range []string{"ftp://x/y", "file:///etc/passwd", "not a url", "http://", "https://h/\nx"} {
		if _, err := NormalizeMirrors(primary, []string{bad}); err == nil {
			t.Errorf("%q must be rejected", bad)
		}
	}
	var many []string
	for i := 0; i <= MaxMirrors; i++ {
		many = append(many, fmt.Sprintf("https://m%d.example/f", i))
	}
	if _, err := NormalizeMirrors(primary, many); err == nil {
		t.Fatalf("more than %d mirrors must be rejected", MaxMirrors)
	}
}

func TestMirrorsPersistRoundTrip(t *testing.T) {
	j := &Job{ID: "1", URL: "http://a/f", Status: StatusPaused,
		Mirrors: []string{"http://b/f", "http://c/f"}, ActiveMirror: "b"}
	data, err := json.Marshal(j)
	if err != nil {
		t.Fatal(err)
	}
	var back Job
	if err := json.Unmarshal(data, &back); err != nil {
		t.Fatal(err)
	}
	if strings.Join(back.GetMirrors(), ",") != "http://b/f,http://c/f" || back.GetActiveMirror() != "b" {
		t.Fatalf("lost on round trip: %v %q", back.Mirrors, back.ActiveMirror)
	}
	plain, _ := json.Marshal(&Job{ID: "2", URL: "http://a/f"})
	if strings.Contains(string(plain), "mirror") {
		t.Fatalf("empty mirrors must be omitted: %s", plain)
	}
}

func TestSetJobMirrorsPersistsToDisk(t *testing.T) {
	m, _ := newManagerForTest(t, 1)
	defer m.Shutdown()
	j := &Job{ID: "job-m", URL: "http://a.example/f", Status: StatusPaused}
	m.mu.Lock()
	m.jobs[j.ID] = j
	m.urls[j.URL] = j.ID
	m.mu.Unlock()

	if _, err := m.SetJobMirrors(j.ID, []string{"http://b.example/f", "http://b.example/f"}); err != nil {
		t.Fatal(err)
	}
	var found *Job
	for _, dj := range readJobsFile() {
		if dj.ID == "job-m" {
			found = dj
		}
	}
	if found == nil || len(found.GetMirrors()) != 1 || found.GetMirrors()[0] != "http://b.example/f" {
		t.Fatalf("mirrors not persisted: %+v", found)
	}
	if _, err := m.SetJobMirrors("nope", nil); err == nil {
		t.Fatal("unknown job must error")
	}
	if _, err := m.SetJobMirrors(j.ID, []string{"ftp://x/y"}); err == nil {
		t.Fatal("bad mirror must error")
	}
	if got, err := m.SetJobMirrors(j.ID, nil); err != nil || len(got) != 0 {
		t.Fatalf("clearing mirrors: %v %v", got, err)
	}
}
