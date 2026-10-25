package download

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/tiredbooy/Rum/backend/internal/pkg/config"
	"github.com/tiredbooy/Rum/backend/internal/pkg/ytdlp"
	"github.com/tiredbooy/Rum/backend/internal/pkg/ytdlp/fakeytdlp"
)

// videoManager is a manager whose yt-dlp is this test binary acting as the fake.
func videoManager(t *testing.T, mode string) (*JobManager, string) {
	t.Helper()
	m, outDir := newManagerForTest(t, 2)
	t.Cleanup(m.Shutdown)
	t.Setenv(fakeytdlp.Env, "1")
	t.Setenv(fakeytdlp.ModeEnv, mode)
	var s config.Setting
	_ = s.LoadSettingMetadata()
	s.YtdlpPath = os.Args[0]
	s.OutDir = outDir
	m.ApplySettings(s)
	return m, outDir
}

func newVideoJob(t *testing.T, m *JobManager) *Job {
	t.Helper()
	job, err := m.CreateVideoJob(VideoJobRequest{
		URL: "https://example.com/watch?v=abc", Title: "Test Video: 100% real/fake", Format: "best", EstSize: 20000000,
	})
	if err != nil {
		t.Fatal(err)
	}
	return job
}

func waitVideoStatus(t *testing.T, j *Job, want string) {
	t.Helper()
	deadline := time.Now().Add(15 * time.Second)
	for time.Now().Before(deadline) {
		if j.GetStatus() == want {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("status = %s (err %v), want %s", j.GetStatus(), j.GetError(), want)
}

func TestVideoJobCompletes(t *testing.T) {
	m, out := videoManager(t, "")
	job := newVideoJob(t, m)
	ch := m.Subscribe(job.ID)
	if err := m.StartJob(context.Background(), job.ID); err != nil {
		t.Fatal(err)
	}
	waitVideoStatus(t, job, StatusCompleted)

	want := filepath.Join(out, "Test Video_ 100_ real_fake.mp4")
	if job.GetOutputPath() != want {
		t.Fatalf("output = %q, want %q", job.GetOutputPath(), want)
	}
	if _, err := os.Stat(want); err != nil {
		t.Fatal(err)
	}
	if job.GetDownloaded() == 0 || job.GetDownloaded() != job.GetTotalSize() {
		t.Fatalf("downloaded %d / total %d", job.GetDownloaded(), job.GetTotalSize())
	}
	// Progress was streamed, mapped onto the normal job progress and never went back.
	last, saw := -1, 0
	for drain := true; drain; {
		select {
		case u := <-ch:
			if u.Progress < last {
				t.Fatalf("progress went back %d -> %d", last, u.Progress)
			}
			last, saw = u.Progress, saw+1
		default:
			drain = false
		}
	}
	if saw == 0 || last != 100 {
		t.Fatalf("progress frames=%d last=%d", saw, last)
	}
}

func TestVideoJobPauseResume(t *testing.T) {
	m, out := videoManager(t, "slow")
	job := newVideoJob(t, m)
	if err := m.StartJob(context.Background(), job.ID); err != nil {
		t.Fatal(err)
	}
	waitVideoStatus(t, job, StatusRunning)
	for job.GetDownloaded() == 0 {
		time.Sleep(10 * time.Millisecond)
	}
	if err := m.PauseJob(job.ID); err != nil {
		t.Fatal(err)
	}
	waitVideoStatus(t, job, StatusPaused)
	part := filepath.Join(out, "Test Video_ 100_ real_fake.mp4.part")
	if _, err := os.Stat(part); err != nil {
		t.Fatalf("partial must survive a pause: %v", err)
	}

	t.Setenv(fakeytdlp.ModeEnv, "") // network is back: the next run finishes
	if err := m.StartJob(context.Background(), job.ID); err != nil {
		t.Fatal(err)
	}
	waitVideoStatus(t, job, StatusCompleted)
	if _, err := os.Stat(part); !os.IsNotExist(err) {
		t.Fatal("part file left after resume")
	}
}

func TestVideoJobDeleteRemovesPartials(t *testing.T) {
	m, out := videoManager(t, "slow")
	job := newVideoJob(t, m)
	if err := m.StartJob(context.Background(), job.ID); err != nil {
		t.Fatal(err)
	}
	for job.GetDownloaded() == 0 {
		time.Sleep(10 * time.Millisecond)
	}
	if err := m.DeleteJob(job.ID); err != nil {
		t.Fatal(err)
	}
	time.Sleep(200 * time.Millisecond)
	if ents, _ := os.ReadDir(out); len(ents) != 0 {
		t.Fatalf("leftovers after cancel: %v", ents)
	}
}

func TestVideoJobErrorCodes(t *testing.T) {
	m, _ := videoManager(t, "unavailable")
	job := newVideoJob(t, m)
	_ = m.StartJob(context.Background(), job.ID)
	waitVideoStatus(t, job, StatusError)
	if code, _ := job.GetErrorCode(); code != "video_unavailable" {
		t.Fatalf("code = %q", code)
	}

	t.Setenv(fakeytdlp.ModeEnv, "fail")
	if err := m.RetryJob(context.Background(), job.ID); err != nil {
		t.Fatal(err)
	}
	waitVideoStatus(t, job, StatusError)
	if code, _ := job.GetErrorCode(); code != "ytdlp_failed" {
		t.Fatalf("code = %q", code)
	}
}

func TestVideoJobMissingTool(t *testing.T) {
	m, _ := videoManager(t, "")
	var s config.Setting
	_ = s.LoadSettingMetadata()
	s.YtdlpPath = ""
	m.ApplySettings(s)
	t.Setenv("PATH", t.TempDir())
	job := newVideoJob(t, m)
	_ = m.StartJob(context.Background(), job.ID)
	waitVideoStatus(t, job, StatusError)
	if code, _ := job.GetErrorCode(); code != ytdlp.CodeMissing {
		t.Fatalf("code = %q", code)
	}
}

func TestCreateVideoJobValidation(t *testing.T) {
	m, _ := videoManager(t, "")
	bad := []VideoJobRequest{
		{URL: "file:///etc/passwd", Title: "x", Format: "best"},
		{URL: "--exec=id", Title: "x", Format: "best"},
		{URL: "https://example.com/v", Title: "x", Format: "best; rm -rf"},
		{URL: "https://example.com/v", Title: "x", Format: ""},
		{URL: "https://example.com/v", Title: "x", Format: "best", DestPath: "../escape"},
	}
	for _, r := range bad {
		if _, err := m.CreateVideoJob(r); err == nil {
			t.Errorf("accepted %+v", r)
		}
	}
	newVideoJob(t, m)
	if _, err := m.CreateVideoJob(VideoJobRequest{URL: "https://example.com/watch?v=abc", Title: "again", Format: "best"}); err == nil {
		t.Error("same link added twice")
	}
}

func TestVideoJobPersistsAcrossRestart(t *testing.T) {
	m, _ := videoManager(t, "")
	job := newVideoJob(t, m)
	raw, err := json.Marshal(job)
	if err != nil {
		t.Fatal(err)
	}
	var back Job
	if err := json.Unmarshal(raw, &back); err != nil {
		t.Fatal(err)
	}
	if back.Kind != JobKindVideo || back.VideoFormat != "best" || back.OutputPath != job.OutputPath || back.TotalSize != 20000000 {
		t.Fatalf("lost on reload: kind=%q fmt=%q out=%q total=%d", back.Kind, back.VideoFormat, back.OutputPath, back.TotalSize)
	}
}

func TestClassifyJobErrorYtdlp(t *testing.T) {
	code, retry := ClassifyJobError(&ytdlp.Error{Code: ytdlp.CodeVideoUnavailable, Message: "Video unavailable"})
	if code != ErrCodeVideoUnavailable || retry {
		t.Fatalf("%s %v", code, retry)
	}
	if ErrCodeYtdlpMissing != "ytdlp_missing" || ErrCodeYtdlpFailed != "ytdlp_failed" {
		t.Fatal("code constants drifted")
	}
}
