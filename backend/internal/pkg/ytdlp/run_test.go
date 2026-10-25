package ytdlp

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func spec(t *testing.T, bin string) Spec {
	return Spec{Bin: bin, URL: "https://example.com/v", Dir: t.TempDir(), Base: "My Video", Selector: "b"}
}

func TestRunCompletesAndReportsProgress(t *testing.T) {
	s := spec(t, fakeBin(t, ""))
	var reports []Progress
	out, err := Run(context.Background(), s, func(p Progress) { reports = append(reports, p) })
	if err != nil {
		t.Fatal(err)
	}
	if out != filepath.Join(s.Dir, "My Video.mp4") {
		t.Fatalf("out = %s", out)
	}
	if _, err := os.Stat(out); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(out + ".part"); !os.IsNotExist(err) {
		t.Fatal(".part left behind")
	}
	if len(reports) != 20 || reports[len(reports)-1].Status != "finished" {
		t.Fatalf("%d reports, last %+v", len(reports), reports[len(reports)-1])
	}
}

func TestRunMapsFailures(t *testing.T) {
	for mode, code := range map[string]string{"unavailable": CodeVideoUnavailable, "fail": CodeFailed} {
		_, err := Run(context.Background(), spec(t, fakeBin(t, mode)), nil)
		if e, ok := AsError(err); !ok || e.Code != code {
			t.Errorf("%s: err = %v", mode, err)
		}
	}
	s := spec(t, filepath.Join(t.TempDir(), "absent"))
	if e, ok := AsError(func() error { _, err := Run(context.Background(), s, nil); return err }()); !ok || e.Code != CodeMissing {
		t.Errorf("absent binary: %v", e)
	}
	s = spec(t, fakeBin(t, ""))
	s.URL = "file:///etc/passwd"
	if _, err := Run(context.Background(), s, nil); err == nil {
		t.Error("file:// URL accepted")
	}
}

// Pause = cancel: the process dies, the .part stays, and a second run resumes.
func TestRunPauseKeepsPartialAndResumes(t *testing.T) {
	s := spec(t, fakeBin(t, "slow"))
	ctx, cancel := context.WithCancel(context.Background())
	got := make(chan struct{}, 1)
	done := make(chan error, 1)
	go func() {
		_, err := Run(ctx, s, func(Progress) {
			select {
			case got <- struct{}{}:
			default:
			}
		})
		done <- err
	}()
	select {
	case <-got:
	case <-time.After(10 * time.Second):
		t.Fatal("no progress")
	}
	cancel()
	select {
	case err := <-done:
		if err != context.Canceled {
			t.Fatalf("err = %v, want context.Canceled", err)
		}
	case <-time.After(10 * time.Second):
		t.Fatal("process not killed")
	}
	part := filepath.Join(s.Dir, "My Video.mp4.part")
	if _, err := os.Stat(part); err != nil {
		t.Fatalf("partial missing after pause: %v", err)
	}
	// Resume with a fast run: the fake notices the .part and completes.
	s.Bin = fakeBin(t, "")
	if _, err := Run(context.Background(), s, nil); err != nil {
		t.Fatal(err)
	}
}

func TestCleanPartials(t *testing.T) {
	dir := t.TempDir()
	touch := func(n string) { os.WriteFile(filepath.Join(dir, n), []byte("x"), 0o644) }
	for _, n := range []string{"v.mp4.part", "v.f137.mp4", "v.f140.m4a.part", "v.f137.mp4.part-Frag3", "v.ytdl", "v.mp4", "v.fr.srt", "other.mp4.part", "vv.mp4.part"} {
		touch(n)
	}
	if n := CleanPartials(dir, "v"); n != 5 {
		t.Fatalf("removed %d, want 5", n)
	}
	for _, keep := range []string{"v.mp4", "v.fr.srt", "other.mp4.part", "vv.mp4.part"} {
		if _, err := os.Stat(filepath.Join(dir, keep)); err != nil {
			t.Errorf("%s was removed", keep)
		}
	}
}
