package download

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
)

// finishedJob registers a completed job whose file holds "hello rum" and whose
// URL points at srv.
func finishedJob(t *testing.T, m *JobManager, srvURL string) (*Job, string) {
	t.Helper()
	p := filepath.Join(t.TempDir(), "a.zip")
	if err := os.WriteFile(p, []byte("hello rum"), 0o644); err != nil {
		t.Fatal(err)
	}
	job := &Job{ID: "j1", URL: srvURL + "/dir/a.zip", FileName: "a.zip", OutputPath: p, Status: StatusCompleted, Downloaded: 9, TotalSize: 9}
	m.mu.Lock()
	m.jobs[job.ID] = job
	m.mu.Unlock()
	return job, p
}

func TestSettleChecksumAutoVerified(t *testing.T) {
	m := newQuietManager(t)
	srv := newFileServer(t, map[string]string{"/dir/a.zip.sha256": sha256Hex([]byte("hello rum")) + "  a.zip"}, nil)
	job, _ := finishedJob(t, m, srv.URL)

	m.settleChecksum(context.Background(), job, Options{Downloader: NewDownloader("", "")}, nil)

	st, algo, src := job.GetChecksumResult()
	if st != ChecksumVerified || algo != "sha256" || src != ChecksumSourceSidecar {
		t.Fatalf("got %q %q %q", st, algo, src)
	}
}

func TestSettleChecksumAutoMismatchKeepsFile(t *testing.T) {
	m := newQuietManager(t)
	srv := newFileServer(t, map[string]string{"/dir/a.zip.sha256": sha256Hex([]byte("other")) + "  a.zip"}, nil)
	job, p := finishedJob(t, m, srv.URL)

	m.settleChecksum(context.Background(), job, Options{Downloader: NewDownloader("", "")}, nil)

	if st, _, _ := job.GetChecksumResult(); st != ChecksumMismatch {
		t.Fatalf("status = %q, want mismatch", st)
	}
	if job.GetStatus() != StatusCompleted {
		t.Errorf("job status = %s, want completed", job.GetStatus())
	}
	if _, err := os.Stat(p); err != nil {
		t.Fatalf("file must stay on disk: %v", err)
	}
}

func TestSettleChecksumNothingFoundIsSilent(t *testing.T) {
	m := newQuietManager(t)
	srv := newFileServer(t, nil, nil)
	job, _ := finishedJob(t, m, srv.URL)

	m.settleChecksum(context.Background(), job, Options{Downloader: NewDownloader("", "")}, nil)

	if st, _, _ := job.GetChecksumResult(); st != "" {
		t.Fatalf("status = %q, want empty", st)
	}
}

func TestSettleChecksumCanBeDisabled(t *testing.T) {
	m := newQuietManager(t)
	srv := newFileServer(t, map[string]string{"/dir/a.zip.sha256": sha256Hex([]byte("hello rum"))}, nil)
	job, _ := finishedJob(t, m, srv.URL)

	m.settleChecksum(context.Background(), job, Options{Downloader: NewDownloader("", ""), NoAutoChecksum: true}, nil)

	if st, _, _ := job.GetChecksumResult(); st != "" {
		t.Fatalf("status = %q, want empty", st)
	}
	if len(srv.seen) != 0 {
		t.Fatalf("disabled discovery still hit the server: %v", srv.seen)
	}
}

func TestSettleChecksumUserSuppliedOutcomes(t *testing.T) {
	m := newQuietManager(t)
	job, _ := finishedJob(t, m, "http://example.invalid")

	m.settleChecksum(context.Background(), job, Options{Checksum: "abc", ChecksumAlgo: "sha256"}, nil)
	if st, algo, src := job.GetChecksumResult(); st != ChecksumVerified || algo != "sha256" || src != ChecksumSourceUser {
		t.Fatalf("verified: got %q %q %q", st, algo, src)
	}

	m.settleChecksum(context.Background(), job, Options{Checksum: "abc", ChecksumAlgo: "md5"}, ErrChecksumMismatch)
	if st, algo, _ := job.GetChecksumResult(); st != ChecksumMismatch || algo != "md5" {
		t.Fatalf("mismatch: got %q %q", st, algo)
	}

	// An unrelated failure says nothing about the checksum.
	other, _ := finishedJob(t, m, "http://example.invalid")
	m.settleChecksum(context.Background(), other, Options{Checksum: "abc", ChecksumAlgo: "md5"}, errors.New("boom"))
	if st, _, _ := other.GetChecksumResult(); st != "" {
		t.Fatalf("status = %q, want empty", st)
	}
}

func TestVerifyChecksumJob(t *testing.T) {
	m := newQuietManager(t)
	srv := newFileServer(t, nil, nil)
	job, _ := finishedJob(t, m, srv.URL)

	// Nothing to compare with: no verdict, nothing recorded.
	res, err := m.VerifyChecksumJob(context.Background(), job.ID)
	if err != nil || res.Status != ChecksumUnavailable {
		t.Fatalf("got %+v err=%v", res, err)
	}
	if st, _, _ := job.GetChecksumResult(); st != "" {
		t.Fatalf("recorded %q for an unavailable checksum", st)
	}

	// User checksum that matches.
	job.SetChecksum(sha256Hex([]byte("hello rum")))
	job.SetChecksumAlgo("sha256")
	res, err = m.VerifyChecksumJob(context.Background(), job.ID)
	if err != nil || res.Status != ChecksumVerified || res.Algo != "sha256" {
		t.Fatalf("got %+v err=%v", res, err)
	}

	// ...and one that does not: reported, file kept.
	job.SetChecksum(sha256Hex([]byte("nope")))
	res, err = m.VerifyChecksumJob(context.Background(), job.ID)
	if err != nil || res.Status != ChecksumMismatch {
		t.Fatalf("got %+v err=%v", res, err)
	}
	if st, _, _ := job.GetChecksumResult(); st != ChecksumMismatch {
		t.Fatalf("job status = %q", st)
	}
	if _, err := os.Stat(job.GetOutputPath()); err != nil {
		t.Fatalf("file must stay: %v", err)
	}

	if _, err := m.VerifyChecksumJob(context.Background(), "missing"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("err = %v, want ErrNotFound", err)
	}
}

func TestVerifyChecksumJobDiscovers(t *testing.T) {
	m := newQuietManager(t)
	srv := newFileServer(t, map[string]string{"/dir/a.zip.md5": md5Hex([]byte("hello rum"))}, nil)
	job, _ := finishedJob(t, m, srv.URL)

	res, err := m.VerifyChecksumJob(context.Background(), job.ID)
	if err != nil || res.Status != ChecksumVerified || res.Algo != "md5" || res.Source != ChecksumSourceSidecar {
		t.Fatalf("got %+v err=%v", res, err)
	}
}

func TestRedownloadKeepsURLAndOptions(t *testing.T) {
	m := newQuietManager(t)
	job, p := finishedJob(t, m, "http://example.invalid")
	job.SetChecksum("abc")
	job.SetChecksumAlgo("md5")
	job.SetChecksumResult(ChecksumMismatch, "md5", ChecksumSourceUser)
	job.SetCategory("Docs")

	if err := m.RedownloadJob(context.Background(), job.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(p); !os.IsNotExist(err) {
		t.Fatalf("damaged file should be replaced, stat err = %v", err)
	}
	if job.GetStatus() != StatusPending || job.GetDownloaded() != 0 {
		t.Errorf("status=%s downloaded=%d", job.GetStatus(), job.GetDownloaded())
	}
	if st, _, _ := job.GetChecksumResult(); st != "" {
		t.Errorf("stale checksum result %q", st)
	}
	if job.GetURL() != "http://example.invalid/dir/a.zip" || job.GetChecksum() != "abc" || job.GetCategory() != "Docs" {
		t.Errorf("url/options lost: %s %s %s", job.GetURL(), job.GetChecksum(), job.GetCategory())
	}

	running := &Job{ID: "r", Status: StatusRunning}
	m.mu.Lock()
	m.jobs["r"] = running
	m.mu.Unlock()
	if err := m.RedownloadJob(context.Background(), "r"); !errors.Is(err, ErrConflict) {
		t.Fatalf("running job: err = %v, want ErrConflict", err)
	}
	if err := m.RedownloadJob(context.Background(), "nope"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("missing job: err = %v, want ErrNotFound", err)
	}
}

func TestChecksumResultPersists(t *testing.T) {
	j := &Job{ID: "p", URL: "http://x/y"}
	j.SetChecksumResult(ChecksumVerified, "sha256", ChecksumSourceHeader)
	data, err := j.MarshalJSON()
	if err != nil {
		t.Fatal(err)
	}
	var back Job
	if err := back.UnmarshalJSON(data); err != nil {
		t.Fatal(err)
	}
	if st, algo, src := back.GetChecksumResult(); st != ChecksumVerified || algo != "sha256" || src != ChecksumSourceHeader {
		t.Fatalf("got %q %q %q", st, algo, src)
	}
}

func TestKeepsFileOnChecksumMismatch(t *testing.T) {
	if !keepsFileOnFailure(ErrChecksumMismatch) {
		t.Error("a checksum mismatch must never delete the file")
	}
	if keepsFileOnFailure(errors.New("boom")) {
		t.Error("ordinary failures follow KeepPartialOnFailure")
	}
}

// End to end through the manager: a download whose published checksum does not
// match completes, is flagged, and the file stays; a matching one is verified.
func TestManagerRunFlagsPublishedChecksum(t *testing.T) {
	for _, tc := range []struct {
		name, published, want string
	}{
		{"mismatch", sha256Hex([]byte("something else")), ChecksumMismatch},
		{"verified", sha256Hex([]byte("hello rum")), ChecksumVerified},
	} {
		t.Run(tc.name, func(t *testing.T) {
			m, _ := newManagerForTest(t, 1)
			defer m.Shutdown()
			srv := newFileServer(t, map[string]string{"/dir/a.zip.sha256": tc.published + "  a.zip"}, nil)

			job := &Job{ID: "e1", URL: srv.URL + "/dir/a.zip", FileName: "a.zip", TotalSize: 9,
				ContentType: "application/zip", Status: StatusPending}
			m.mu.Lock()
			m.jobs[job.ID] = job
			m.mu.Unlock()

			done := make(chan struct{}, 2)
			prev := setTestCompletionHook(func(string) { done <- struct{}{} })
			defer setTestCompletionHook(prev)
			if err := m.StartJob(context.Background(), job.ID); err != nil {
				t.Fatal(err)
			}
			<-done

			if job.GetStatus() != StatusCompleted {
				t.Fatalf("status=%s err=%v", job.GetStatus(), job.GetError())
			}
			if st, _, _ := job.GetChecksumResult(); st != tc.want {
				t.Fatalf("checksum status = %q, want %q", st, tc.want)
			}
			if _, err := os.Stat(job.GetOutputPath()); err != nil {
				t.Fatalf("file missing: %v", err)
			}
		})
	}
}

// A user-supplied checksum that does not match fails the job with the checksum
// code, flags it as a mismatch, and keeps the file even when KeepPartialOnFailure
// is off.
func TestManagerRunUserChecksumMismatchKeepsFile(t *testing.T) {
	m, _ := newManagerForTest(t, 1)
	defer m.Shutdown()
	m.opt.KeepPartialOnFailure = false
	srv := newFileServer(t, nil, nil)

	job := &Job{ID: "e2", URL: srv.URL + "/dir/a.zip", FileName: "a.zip", TotalSize: 9,
		ContentType: "application/zip", Status: StatusPending,
		Checksum: sha256Hex([]byte("nope")), ChecksumAlgo: "sha256"}
	m.mu.Lock()
	m.jobs[job.ID] = job
	m.mu.Unlock()

	done := make(chan struct{}, 2)
	prev := setTestCompletionHook(func(string) { done <- struct{}{} })
	defer setTestCompletionHook(prev)
	if err := m.StartJob(context.Background(), job.ID); err != nil {
		t.Fatal(err)
	}
	<-done

	if code, _ := job.GetErrorCode(); job.GetStatus() != StatusError || code != ErrCodeChecksum {
		t.Fatalf("status=%s code=%q err=%v", job.GetStatus(), code, job.GetError())
	}
	if st, _, src := job.GetChecksumResult(); st != ChecksumMismatch || src != ChecksumSourceUser {
		t.Fatalf("checksum result = %q/%q", st, src)
	}
	if _, err := os.Stat(job.GetOutputPath()); err != nil {
		t.Fatalf("mismatching file was deleted: %v", err)
	}
}
