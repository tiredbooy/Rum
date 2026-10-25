package download

import (
	"encoding/json"
	"strings"
	"testing"
)

func seedJob(m *JobManager, id, url, dest, status string) *Job {
	j := &Job{ID: id, URL: url, FileName: id + ".bin", DestPath: dest, Status: status,
		Category: "video", Checksum: "abc", ChecksumAlgo: "sha256", SpeedLimit: 50, MaxRetries: 3,
		Referer: "http://secret.example/?token=SECRET", UserAgent: "UA-SECRET"}
	m.mu.Lock()
	m.jobs[id] = j
	m.urls[url] = id
	m.mu.Unlock()
	return j
}

func TestExportOmitsSecretsAndKeepsOptions(t *testing.T) {
	m := newTestManager(t, &Options{Parallel: 1})
	seedJob(m, "a", "http://x.test/a.bin", "/tmp/d", StatusPaused)

	raw, err := json.Marshal(m.ExportDownloads())
	if err != nil {
		t.Fatal(err)
	}
	s := string(raw)
	for _, bad := range []string{"SECRET", "referer", "user_agent", "cookie"} {
		if strings.Contains(strings.ToLower(s), strings.ToLower(bad)) {
			t.Fatalf("export leaked %q: %s", bad, s)
		}
	}
	for _, want := range []string{"http://x.test/a.bin", "a.bin", "/tmp/d", "video", "sha256"} {
		if !strings.Contains(s, want) {
			t.Fatalf("export missing %q: %s", want, s)
		}
	}
}

func TestImportRoundtripAddsPausedAndDedupes(t *testing.T) {
	src := newTestManager(t, &Options{Parallel: 1})
	seedJob(src, "a", "http://x.test/a.bin", "/tmp/d", StatusPaused)
	seedJob(src, "b", "http://x.test/b.bin", "", StatusRunning)
	seedJob(src, "c", "http://x.test/c.bin", "", StatusCompleted)
	doc := src.ExportDownloads()

	dst := newTestManager(t, &Options{Parallel: 1, Out: t.TempDir()})
	res := dst.ImportDownloads(doc)
	if res.Added != 2 || res.Skipped != 1 {
		t.Fatalf("first import: %+v", res)
	}
	var a *Job
	for _, j := range dst.GetAllJobs() {
		if j.GetStatus() != StatusPaused {
			t.Fatalf("imported job must be paused, got %s", j.GetStatus())
		}
		if j.GetURL() == "http://x.test/a.bin" {
			a = j
		}
	}
	if a == nil || a.GetDestPath() != "/tmp/d" || a.GetCategory() != "video" || a.GetFileName() != "a.bin" {
		t.Fatalf("options lost on import: %+v", a)
	}

	again := dst.ImportDownloads(doc)
	if again.Added != 0 || len(dst.GetAllJobs()) != 2 {
		t.Fatalf("re-import must dedupe by URL+dest: %+v", again)
	}

	// Same URL, different dest is a different download.
	doc2 := ExportDoc{Version: 1, Downloads: []ExportedDownload{{URL: "http://x.test/a.bin", DestPath: "/tmp/other"}}}
	if r := dst.ImportDownloads(doc2); r.Added != 1 {
		t.Fatalf("different dest should import: %+v", r)
	}
}

func TestImportRejectsBadItems(t *testing.T) {
	m := newTestManager(t, &Options{Parallel: 1, Out: t.TempDir()})
	res := m.ImportDownloads(ExportDoc{Version: 1, Downloads: []ExportedDownload{
		{URL: "file:///etc/passwd"},
		{URL: "http://x.test/ok.bin", DestPath: "../escape"},
		{URL: "http://x.test/ok.bin"},
		{URL: "http://x.test/ok.bin"},
	}})
	if res.Invalid != 2 || res.Added != 1 || res.Skipped != 1 {
		t.Fatalf("got %+v", res)
	}
}
