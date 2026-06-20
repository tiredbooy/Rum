package download

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"
)

// ---------------------------------------------------------------------------
// Controllable range server for reliability tests.
//
// It serves a payload over HTTP honoring Range + If-Range exactly as a real CDN
// would, and exposes knobs to simulate the failure modes from the design doc:
//   - changing the ETag between sessions (validator drift, C1),
//   - dropping the connection mid-stream (transient fault),
//   - returning a 200 to an If-Range whose validator no longer matches.
// ---------------------------------------------------------------------------

type ctrlServer struct {
	mu       atomic.Pointer[ctrlState]
	requests atomic.Int64
}

type ctrlState struct {
	payload []byte
	etag    string
	// dropAfter, when > 0, truncates each 206 body to this many bytes then closes
	// the connection, simulating a mid-stream disconnect. 0 disables it.
	dropAfter int
}

func newCtrlServer(payload []byte, etag string) (*httptest.Server, *ctrlServer) {
	cs := &ctrlServer{}
	cs.mu.Store(&ctrlState{payload: payload, etag: etag})

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		cs.requests.Add(1)
		st := cs.mu.Load()
		w.Header().Set("Accept-Ranges", "bytes")
		w.Header().Set("ETag", st.etag)

		rangeHdr := r.Header.Get("Range")
		ifRange := r.Header.Get("If-Range")

		// If-Range validator no longer matches -> RFC 7233 says ignore Range and
		// send the full 200 body. This is the "source changed" signal (E1).
		if ifRange != "" && normalizeValidator(ifRange) != normalizeValidator(st.etag) {
			w.Header().Set("Content-Length", strconv.Itoa(len(st.payload)))
			w.WriteHeader(http.StatusOK)
			w.Write(st.payload)
			return
		}

		if rangeHdr == "" {
			w.Header().Set("Content-Length", strconv.Itoa(len(st.payload)))
			w.WriteHeader(http.StatusOK)
			w.Write(st.payload)
			return
		}

		start, end, ok := parseTestRange(rangeHdr, len(st.payload))
		if !ok {
			w.WriteHeader(http.StatusRequestedRangeNotSatisfiable)
			return
		}
		w.Header().Set("Content-Range", fmt.Sprintf("bytes %d-%d/%d", start, end, len(st.payload)))
		w.Header().Set("Content-Length", strconv.Itoa(end-start+1))
		w.WriteHeader(http.StatusPartialContent)

		body := st.payload[start : end+1]
		if st.dropAfter > 0 && len(body) > st.dropAfter {
			body = body[:st.dropAfter]
			// Write a truncated body and abort the connection so the client sees an
			// unexpected EOF mid-stream (a retryable transient fault).
			w.Write(body)
			if f, ok := w.(http.Flusher); ok {
				f.Flush()
			}
			if hj, ok := w.(http.Hijacker); ok {
				if conn, _, err := hj.Hijack(); err == nil {
					conn.Close()
				}
			}
			return
		}
		w.Write(body)
	}))
	return srv, cs
}

// setETag swaps the served ETag (and optionally payload) to simulate a re-upload.
func (cs *ctrlServer) setState(payload []byte, etag string, dropAfter int) {
	cs.mu.Store(&ctrlState{payload: payload, etag: etag, dropAfter: dropAfter})
}

func parseTestRange(h string, size int) (start, end int, ok bool) {
	if !strings.HasPrefix(h, "bytes=") {
		return 0, 0, false
	}
	spec := strings.TrimPrefix(h, "bytes=")
	parts := strings.SplitN(spec, "-", 2)
	if len(parts) != 2 {
		return 0, 0, false
	}
	s, err1 := strconv.Atoi(strings.TrimSpace(parts[0]))
	if err1 != nil {
		return 0, 0, false
	}
	if strings.TrimSpace(parts[1]) == "" {
		end = size - 1
	} else {
		e, err2 := strconv.Atoi(strings.TrimSpace(parts[1]))
		if err2 != nil {
			return 0, 0, false
		}
		end = e
	}
	start = s
	if start < 0 || end >= size || start > end {
		return 0, 0, false
	}
	return start, end, true
}

func ctrlJobOpt(t *testing.T, srv *httptest.Server, payload []byte, connections int) (*Job, Options, string) {
	t.Helper()
	dir := t.TempDir()
	job := &Job{
		ID:           "rel-test",
		URL:          srv.URL + "/file.bin",
		FileName:     "file.bin",
		TotalSize:    int64(len(payload)),
		SupportRange: true,
		Status:       StatusPending,
	}
	opt := Options{
		Out:         dir,
		Connections: connections,
		Downloader:  NewDownloader("test-agent", ""),
		MaxRetries:  3,
	}
	return job, opt, dir
}

func assertBytesEqual(t *testing.T, out string, want []byte) {
	t.Helper()
	got, err := os.ReadFile(out)
	if err != nil {
		t.Fatalf("read output: %v", err)
	}
	if len(got) != len(want) {
		t.Fatalf("size mismatch: got %d want %d", len(got), len(want))
	}
	for i := range got {
		if got[i] != want[i] {
			t.Fatalf("byte mismatch at %d: got %d want %d", i, got[i], want[i])
		}
	}
}

// ---------------------------------------------------------------------------
// E1: changed-ETag resume -> clean restart from zero, byte-correct output.
// ---------------------------------------------------------------------------

func TestResumeDetectsChangedETagAndRestarts(t *testing.T) {
	payloadV1 := makePayload(minSegmentedSize * 2)
	srv, cs := newCtrlServer(payloadV1, `"v1"`)
	defer srv.Close()

	job, opt, dir := ctrlJobOpt(t, srv, payloadV1, 4)
	out := filepath.Join(dir, "file.bin")

	// Simulate a prior session that downloaded the first half of every segment of
	// version 1 and persisted a sidecar carrying the v1 ETag.
	segs := computeSegments(int64(len(payloadV1)), 4)
	if err := os.WriteFile(out, make([]byte, len(payloadV1)), 0644); err != nil {
		t.Fatal(err)
	}
	f, err := os.OpenFile(out, os.O_WRONLY, 0644)
	if err != nil {
		t.Fatal(err)
	}
	for i := range segs {
		half := segs[i].size() / 2
		segs[i].Done = half
		if _, err := f.WriteAt(payloadV1[segs[i].Start:segs[i].Start+half], segs[i].Start); err != nil {
			t.Fatal(err)
		}
	}
	f.Close()
	savePartsMeta(out, &partsMeta{
		TotalSize: int64(len(payloadV1)), URL: job.URL, Version: partsMetaVersion,
		ETag: `"v1"`, Segments: segs,
	})

	// The remote was re-uploaded: same size, NEW content + NEW ETag. Resume must
	// detect the validator change (If-Range -> 200), discard the stale partial,
	// and re-download the whole v2 file correctly.
	payloadV2 := makePayload(minSegmentedSize * 2)
	for i := range payloadV2 {
		payloadV2[i] = byte((int(payloadV2[i]) + 7) % 251) // differ from v1
	}
	cs.setState(payloadV2, `"v2"`, 0)

	if err := DownloadSegmented(context.Background(), opt, job, nil); err != nil {
		t.Fatalf("resume-with-changed-etag failed: %v", err)
	}
	// Output must be the FULL v2 payload, not v1-prefix + v2-suffix.
	assertBytesEqual(t, out, payloadV2)
	// Sidecar gone on clean success.
	if _, err := os.Stat(partsFilePath(out)); !os.IsNotExist(err) {
		t.Errorf("expected sidecar removed after successful restart")
	}
}

// ---------------------------------------------------------------------------
// E2: crash-resume leaves no hole. A sidecar that under-reports progress (a
// "crash" lost the in-cache tail) must resume and refill the missing bytes; a
// sidecar that over-reports must never be trusted to skip a hole — but our
// durability ordering guarantees the sidecar never claims more than is on disk,
// so we test the safe direction: partial + correct bytes -> filled, no hole.
// ---------------------------------------------------------------------------

func TestCrashResumeNoHole(t *testing.T) {
	payload := makePayload(minSegmentedSize * 2)
	srv, _ := newCtrlServer(payload, `"v1"`)
	defer srv.Close()

	job, opt, dir := ctrlJobOpt(t, srv, payload, 4)
	out := filepath.Join(dir, "file.bin")

	segs := computeSegments(int64(len(payload)), 4)
	if err := os.WriteFile(out, make([]byte, len(payload)), 0644); err != nil {
		t.Fatal(err)
	}
	f, _ := os.OpenFile(out, os.O_WRONLY, 0644)
	for i := range segs {
		// Only a third done, rest is a hole that resume must fill.
		part := segs[i].size() / 3
		segs[i].Done = part
		f.WriteAt(payload[segs[i].Start:segs[i].Start+part], segs[i].Start)
	}
	f.Close()
	savePartsMeta(out, &partsMeta{
		TotalSize: int64(len(payload)), URL: job.URL, Version: partsMetaVersion,
		ETag: `"v1"`, Segments: segs,
	})

	if err := DownloadSegmented(context.Background(), opt, job, nil); err != nil {
		t.Fatalf("crash-resume failed: %v", err)
	}
	assertBytesEqual(t, out, payload)
}

// ---------------------------------------------------------------------------
// Transient mid-stream disconnect is retried and the file completes correctly.
// ---------------------------------------------------------------------------

func TestSegmentedRetriesMidStreamDrop(t *testing.T) {
	payload := makePayload(minSegmentedSize * 2)
	srv, cs := newCtrlServer(payload, `"v1"`)
	defer srv.Close()

	// Drop every 206 after a few KB for the first burst of requests, then heal so
	// retries can finish. We heal after enough requests for all segments to have
	// failed at least once.
	cs.setState(payload, `"v1"`, 4096)

	job, opt, dir := ctrlJobOpt(t, srv, payload, 4)
	out := filepath.Join(dir, "file.bin")

	// Heal the server shortly after the run starts, in a goroutine, by watching
	// the request counter.
	done := make(chan struct{})
	go func() {
		for {
			if cs.requests.Load() >= 4 {
				cs.setState(payload, `"v1"`, 0) // stop dropping
				close(done)
				return
			}
		}
	}()

	err := DownloadSegmented(context.Background(), opt, job, nil)
	<-done
	if err != nil {
		t.Fatalf("download with mid-stream drops failed: %v", err)
	}
	assertBytesEqual(t, out, payload)
}

// ---------------------------------------------------------------------------
// E3: an injected hole makes completion verification fail and keeps the sidecar.
// We drive verifySegmentsComplete directly (the structural always-verify check)
// to assert it catches a sparse hole that a size-only check would miss.
// ---------------------------------------------------------------------------

func TestVerifySegmentsCompleteCatchesHole(t *testing.T) {
	dir := t.TempDir()
	out := filepath.Join(dir, "file.bin")
	total := int64(minSegmentedSize)
	// Presized (sparse) file: correct SIZE, but no real bytes -> holes.
	if err := os.WriteFile(out, make([]byte, total), 0644); err != nil {
		t.Fatal(err)
	}
	segs := computeSegments(total, 4)
	segDone := make([]atomic.Int64, len(segs))
	for i := range segs {
		segDone[i].Store(segs[i].size())
	}
	// Inject a hole: segment 2 only half written.
	segDone[2].Store(segs[2].size() / 2)

	err := verifySegmentsComplete(segs, segDone, total, out)
	if err == nil {
		t.Fatal("expected verification to fail on an incomplete segment (hole)")
	}
	code, _ := Classify(err)
	if code != codeChecksum {
		t.Errorf("expected checksum-kind error for a hole, got code %q (%v)", code, err)
	}
}

// Full E3 end-to-end: a completed download with VerifyIntegrity stores a full-file
// hash ON THE JOB (persisted in jobs.json) so a later VerifyFile can re-check it
// without the server, and leaves NO *.rumparts sidecar cluttering the user's
// folder. Tampering with the bytes is still caught by the stored hash.
func TestVerifyIntegrityStoresHashOnJobNoSidecar(t *testing.T) {
	payload := makePayload(minSegmentedSize * 2)
	srv, _ := newCtrlServer(payload, `"v1"`)
	defer srv.Close()

	job, opt, dir := ctrlJobOpt(t, srv, payload, 4)
	opt.VerifyIntegrity = true
	out := filepath.Join(dir, "file.bin")

	if err := DownloadSegmented(context.Background(), opt, job, nil); err != nil {
		t.Fatalf("download failed: %v", err)
	}
	assertBytesEqual(t, out, payload)

	// No sidecar clutter must remain next to a finished download.
	if _, err := os.Stat(partsFilePath(out)); !os.IsNotExist(err) {
		t.Errorf("expected sidecar removed after VerifyIntegrity completion, stat err = %v", err)
	}

	// The integrity hash must be recorded on the job for server-free re-verification.
	if job.GetIntegrityHash() == "" || job.IntegrityHashAlgo != "sha256" {
		t.Fatalf("expected integrity hash on job, got %q/%q", job.GetIntegrityHash(), job.IntegrityHashAlgo)
	}

	// VerifyFile must report ok via the stored hash (no server contact needed).
	res, err := VerifyFile(context.Background(), opt, job, false)
	if err != nil {
		t.Fatalf("VerifyFile error: %v", err)
	}
	if !res.Ok || res.Method != "stored_hash" {
		t.Fatalf("expected ok via stored_hash, got %+v", res)
	}

	// Tampering with the bytes (same length) must be caught via the stored hash.
	tampered := append([]byte{payload[0] ^ 0xFF}, payload[1:]...)
	if err := os.WriteFile(out, tampered, 0644); err != nil {
		t.Fatalf("tamper write: %v", err)
	}
	res, err = VerifyFile(context.Background(), opt, job, false)
	if err != nil {
		t.Fatalf("VerifyFile (tampered) error: %v", err)
	}
	if res.Ok || res.Method != "stored_hash" {
		t.Fatalf("expected not-ok via stored_hash after tamper, got %+v", res)
	}
}

// VerifyFile localizes a hole to the offending segment via the sidecar.
func TestVerifyFileLocalizesHole(t *testing.T) {
	payload := makePayload(minSegmentedSize * 2)
	srv, _ := newCtrlServer(payload, `"v1"`)
	defer srv.Close()

	job, opt, dir := ctrlJobOpt(t, srv, payload, 4)
	out := filepath.Join(dir, "file.bin")

	// Presized file + a sidecar that marks segment 1 incomplete (a hole).
	if err := os.WriteFile(out, make([]byte, len(payload)), 0644); err != nil {
		t.Fatal(err)
	}
	segs := computeSegments(int64(len(payload)), 4)
	for i := range segs {
		segs[i].Done = segs[i].size()
	}
	segs[1].Done = segs[1].size() / 2 // hole in segment 1
	savePartsMeta(out, &partsMeta{
		TotalSize: int64(len(payload)), URL: job.URL, Version: partsMetaVersion,
		ETag: `"v1"`, Segments: segs,
	})

	res, err := VerifyFile(context.Background(), opt, job, false)
	if err != nil {
		t.Fatalf("VerifyFile error: %v", err)
	}
	if res.Ok {
		t.Fatal("expected VerifyFile to report not-ok for a hole")
	}
	if len(res.BadSegments) != 1 || res.BadSegments[0] != 1 {
		t.Fatalf("expected bad segment [1], got %v", res.BadSegments)
	}
}

// ---------------------------------------------------------------------------
// E4: RepairFile re-fetches ONLY the bad segment and produces a byte-correct file.
// ---------------------------------------------------------------------------

func TestRepairFileRefetchesOnlyBadSegment(t *testing.T) {
	payload := makePayload(minSegmentedSize * 2)
	srv, cs := newCtrlServer(payload, `"v1"`)
	defer srv.Close()

	job, opt, dir := ctrlJobOpt(t, srv, payload, 4)
	out := filepath.Join(dir, "file.bin")

	segs := computeSegments(int64(len(payload)), 4)

	// Lay down a file that is correct everywhere EXCEPT segment 2, which is full of
	// zeros (a hole). All other segments hold the true bytes.
	full := make([]byte, len(payload))
	copy(full, payload)
	badIdx := 2
	for i := segs[badIdx].Start; i <= segs[badIdx].End; i++ {
		full[i] = 0
	}
	if err := os.WriteFile(out, full, 0644); err != nil {
		t.Fatal(err)
	}

	// Count requests before repair so we can assert repair only fetched 1 segment.
	before := cs.requests.Load()

	if err := RepairFile(context.Background(), opt, job, []int{badIdx}); err != nil {
		t.Fatalf("RepairFile failed: %v", err)
	}
	after := cs.requests.Load()

	// Only the single bad segment should have been fetched (1 request).
	if got := after - before; got != 1 {
		t.Fatalf("expected exactly 1 segment refetch, got %d requests", got)
	}
	// And the file must now be byte-correct.
	assertBytesEqual(t, out, payload)
}

// RepairFile must refuse to write mismatched bytes if the source changed (E1
// safety reused): the refetch gets a 200 (validator changed) and errors out.
func TestRepairFileRejectsChangedSource(t *testing.T) {
	payload := makePayload(minSegmentedSize * 2)
	srv, cs := newCtrlServer(payload, `"v1"`)
	defer srv.Close()

	job, opt, dir := ctrlJobOpt(t, srv, payload, 4)
	out := filepath.Join(dir, "file.bin")

	segs := computeSegments(int64(len(payload)), 4)
	full := make([]byte, len(payload))
	copy(full, payload)
	for i := segs[1].Start; i <= segs[1].End; i++ {
		full[i] = 0
	}
	if err := os.WriteFile(out, full, 0644); err != nil {
		t.Fatal(err)
	}
	// Sidecar carries the v1 validator; the server now serves v2.
	for i := range segs {
		segs[i].Done = segs[i].size()
	}
	segs[1].Done = 0
	savePartsMeta(out, &partsMeta{
		TotalSize: int64(len(payload)), URL: job.URL, Version: partsMetaVersion,
		ETag: `"v1"`, Segments: segs,
	})
	cs.setState(payload, `"v2"`, 0)

	err := RepairFile(context.Background(), opt, job, []int{1})
	if err == nil {
		t.Fatal("expected RepairFile to fail when the source validator changed")
	}
	if !errors.Is(err, errValidatorChanged) {
		t.Fatalf("expected errValidatorChanged, got %v", err)
	}
}
