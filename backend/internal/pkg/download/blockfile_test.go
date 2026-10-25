package download

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"sync/atomic"
	"testing"
)

func assertNoOutput(t *testing.T, path string) {
	t.Helper()
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatalf("output file must not exist, stat err=%v", err)
	}
	if _, err := os.Stat(partsFilePath(path)); !os.IsNotExist(err) {
		t.Fatalf("sidecar must not exist, stat err=%v", err)
	}
}

func TestSegmentedBlockPageLeavesNoFile(t *testing.T) {
	total := int64(minSegmentedSize) * 2
	f := newBlockFixture(t, "big.zip", total, nil, htmlHandler("text/html"))
	f.opt.Connections = 4
	err := DownloadSegmented(context.Background(), f.opt, f.job, nil)
	var bp *blockedPageError
	if !errors.As(err, &bp) {
		t.Fatalf("want blockedPageError, got %v", err)
	}
	assertNoOutput(t, f.path)
}

// binaryThenPortal serves a valid 206 to the first request and an HTML page to
// every later one, so detection can only happen after the file is allocated.
func binaryThenPortal(total int64) http.HandlerFunc {
	var n atomic.Int32
	return func(w http.ResponseWriter, r *http.Request) {
		if n.Add(1) == 1 {
			body := makePayload(512)
			w.Header().Set("Content-Type", "application/octet-stream")
			w.Header().Set("Content-Range", fmt.Sprintf("bytes 0-511/%d", total))
			w.WriteHeader(http.StatusPartialContent)
			_, _ = w.Write(body)
			return
		}
		htmlHandler("text/html")(w, r)
	}
}

func TestSegmentedLateBlockPageRemovesCreatedFile(t *testing.T) {
	total := int64(minSegmentedSize) * 2
	f := newBlockFixture(t, "late.zip", total, nil, binaryThenPortal(total))
	f.opt.Connections = 4
	err := DownloadSegmented(context.Background(), f.opt, f.job, nil)
	var bp *blockedPageError
	if !errors.As(err, &bp) {
		t.Fatalf("want blockedPageError, got %v", err)
	}
	assertNoOutput(t, f.path)
}

func TestSegmentedBlockPageKeepsPreexistingFile(t *testing.T) {
	total := int64(minSegmentedSize) * 2
	for name, h := range map[string]func(int64) http.HandlerFunc{
		"early": func(int64) http.HandlerFunc { return htmlHandler("text/html") },
		"late":  binaryThenPortal,
	} {
		keep := []byte("user data that must survive")
		f := newBlockFixture(t, "mine.zip", total, keep, h(total))
		f.opt.Connections = 4
		err := DownloadSegmented(context.Background(), f.opt, f.job, nil)
		var bp *blockedPageError
		if !errors.As(err, &bp) {
			t.Fatalf("%s: want blockedPageError, got %v", name, err)
		}
		fi, statErr := os.Stat(f.path)
		if statErr != nil {
			t.Fatalf("%s: pre-existing file was removed", name)
		}
		if name == "early" {
			if got, _ := os.ReadFile(f.path); string(got) != string(keep) {
				t.Fatalf("early: pre-existing file modified (%d bytes)", fi.Size())
			}
		}
	}
}

func TestBlockedPage206GenericTypeSniffed(t *testing.T) {
	partial := makePayload(1000)
	f := newBlockFixture(t, "file.zip", 5000, partial, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/octet-stream")
		w.Header().Set("Content-Range", "bytes 1000-1070/5000")
		w.WriteHeader(http.StatusPartialContent)
		_, _ = w.Write([]byte(portalHTML))
	})
	err := DownloadSingleFile(context.Background(), f.opt, f.job, nil)
	var bp *blockedPageError
	if !errors.As(err, &bp) {
		t.Fatalf("want blockedPageError, got %v", err)
	}
	if got, _ := os.ReadFile(f.path); string(got) != string(partial) {
		t.Fatalf("partial modified: %d bytes", len(got))
	}
}

func TestBlockedPage206MissingTypeSniffed(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header()["Content-Type"] = nil
		w.WriteHeader(http.StatusPartialContent)
		_, _ = w.Write([]byte(portalHTML))
	}))
	defer srv.Close()
	resp, err := http.Get(srv.URL)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	job := &Job{FileName: "a.zip", ContentType: "application/zip"}
	var bp *blockedPageError
	if err := checkBlockedPage(job, resp); !errors.As(err, &bp) {
		t.Fatalf("want blockedPageError, got %v", err)
	}
}

func TestTextPlainPortalCaughtForBinaryJob(t *testing.T) {
	for _, body := range []string{portalHTML, "Access denied. Contact your network administrator."} {
		f := newBlockFixture(t, "file.zip", 5000, nil, func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "text/plain; charset=utf-8")
			_, _ = w.Write([]byte(body))
		})
		err := DownloadSingleFile(context.Background(), f.opt, f.job, nil)
		var bp *blockedPageError
		if !errors.As(err, &bp) {
			t.Fatalf("body %.20q: want blockedPageError, got %v", body, err)
		}
		if fi, err := os.Stat(f.path); err == nil && fi.Size() != 0 {
			t.Fatalf("wrote %d bytes", fi.Size())
		}
	}
}

func TestLegitTextPlainDownloadsWork(t *testing.T) {
	const text = "hello, plain text file\nsecond line\n"
	f := newBlockFixture(t, "notes.txt", int64(len(text)), nil, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain")
		_, _ = w.Write([]byte(text))
	})
	f.job.ContentType = "text/plain"
	if err := DownloadSingleFile(context.Background(), f.opt, f.job, nil); err != nil {
		t.Fatal(err)
	}
	if got, _ := os.ReadFile(f.path); string(got) != text {
		t.Fatalf("content mismatch: %q", got)
	}
}

func TestBinaryServedAsTextPlainStillDownloads(t *testing.T) {
	payload := makePayload(4096)
	f := newBlockFixture(t, "file.zip", int64(len(payload)), nil, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain")
		_, _ = w.Write(payload)
	})
	if err := DownloadSingleFile(context.Background(), f.opt, f.job, nil); err != nil {
		t.Fatal(err)
	}
}

func TestSegmentedCancelledBeforeProbeCreatesNoFile(t *testing.T) {
	total := int64(minSegmentedSize) * 2
	f := newBlockFixture(t, "cancel.zip", total, nil, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/octet-stream")
		_, _ = w.Write(makePayload(1024))
	})
	f.opt.Connections = 4
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := DownloadSegmented(ctx, f.opt, f.job, nil); err == nil {
		t.Fatal("want error")
	}
	if _, err := os.Stat(f.path); !os.IsNotExist(err) {
		t.Fatalf("cancelled job must not allocate output, stat err=%v", err)
	}
}
