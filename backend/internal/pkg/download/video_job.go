package download

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/tiredbooy/Rum/backend/internal/pkg/config"
	"github.com/tiredbooy/Rum/backend/internal/pkg/ytdlp"
)

// JobKindVideo marks a job that downloads a video page through yt-dlp.
const JobKindVideo = "video"

// IsVideo reports whether the job is a yt-dlp page download.
func (j *Job) IsVideo() bool { j.Mu.RLock(); defer j.Mu.RUnlock(); return j.Kind == JobKindVideo }

// GetVideoFormat returns the picker key chosen for a video job.
func (j *Job) GetVideoFormat() string { j.Mu.RLock(); defer j.Mu.RUnlock(); return j.VideoFormat }

// VideoJobRequest describes a page download chosen in the format picker.
type VideoJobRequest struct {
	URL        string
	Title      string // used for the file name only; sanitized here
	Format     string // picker key: best | h1080 | h720 | h480 | audio
	EstSize    int64  // best-effort bytes (0 = unknown)
	DestPath   string // like other jobs: relative to the download folder, or absolute
	SpeedLimit int    // KB/s for this job only
}

func videoValidation(msg string) error {
	return &CodedError{Code: codeValidation, Kind: KindValidation, Err: errors.New(msg)}
}

// CreateVideoJob queues a page download. It does no network work: the title and
// size come from the format listing the user just saw. The job is pending; the
// caller starts it.
func (m *JobManager) CreateVideoJob(req VideoJobRequest) (*Job, error) {
	link := strings.TrimSpace(req.URL)
	if err := ValidateURL(link); err != nil {
		return nil, err
	}
	if !ytdlp.ValidKey(req.Format) {
		return nil, videoValidation("invalid format")
	}
	if req.SpeedLimit < 0 {
		return nil, videoValidation("invalid speed limit")
	}
	m.mu.RLock()
	base := m.opt.Out
	m.mu.RUnlock()
	dest, err := ResolveDestPath(base, req.DestPath)
	if err != nil {
		return nil, err
	}
	if dest == "" {
		dest = base
	}
	if dest == "" {
		return nil, videoValidation("no download folder is set")
	}

	m.mu.Lock()
	defer m.mu.Unlock()
	if _, taken := m.urls[link]; taken {
		return nil, fmt.Errorf("this link is already in the list: %w", ErrConflict)
	}
	name := freeVideoName(dest, ytdlp.SafeBase(req.Title))
	job := &Job{
		ID:          uuid.New().String(),
		URL:         link,
		FileName:    name,
		OutputPath:  filepath.Join(dest, name), // no extension until yt-dlp reports the real file
		TotalSize:   req.EstSize,
		ContentType: "video",
		Status:      StatusPending,
		CreatedAt:   time.Now(),
		SpeedLimit:  req.SpeedLimit,
		DestPath:    req.DestPath,
		Kind:        JobKindVideo,
		VideoFormat: req.Format,
	}
	m.jobs[job.ID] = job
	m.urls[job.URL] = job.ID
	m.saveToDiskLocked()
	return job, nil
}

// freeVideoName avoids clobbering files of an earlier download with the same
// title: "name", then "name (2)", ... until nothing in dir starts with "name.".
func freeVideoName(dir, name string) string {
	taken := func(n string) bool {
		entries, err := os.ReadDir(dir)
		if err != nil {
			return false
		}
		for _, e := range entries {
			if strings.HasPrefix(e.Name(), n+".") {
				return true
			}
		}
		return false
	}
	if !taken(name) {
		return name
	}
	for i := 2; i < 1000; i++ {
		if c := fmt.Sprintf("%s (%d)", name, i); !taken(c) {
			return c
		}
	}
	return name
}

// YtdlpSettings is the slice of settings video features need: the configured
// yt-dlp path (may be empty) and the normalized proxy URL (may be empty).
func (m *JobManager) YtdlpSettings() (configuredPath, proxy string) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	proxy, _ = config.NormalizeProxy(m.config.Proxy)
	return m.config.YtdlpPath, proxy
}

// runVideo downloads a video job with yt-dlp. Cancelling ctx kills the process
// (pause) and keeps the .part files, which `--continue` picks up on the next run.
func (m *JobManager) runVideo(ctx context.Context, opt Options, job *Job, progressFn func(downloaded, total int64)) error {
	configured, proxy := m.YtdlpSettings()
	found, err := ytdlp.Locate(configured)
	if err != nil {
		return err
	}
	out := job.GetOutputPath()
	if out == "" {
		return videoValidation("video job has no output folder")
	}
	dir, base := filepath.Dir(out), filepath.Base(out)
	if job.GetStatus() == StatusCompleted { // defensive: a finished job has an extension
		base = strings.TrimSuffix(base, filepath.Ext(base))
	}

	spec := ytdlp.Spec{
		Bin:       found.Path,
		URL:       job.GetURL(),
		Dir:       dir,
		Base:      base,
		Selector:  ytdlp.Selector(job.GetVideoFormat(), ytdlp.HasFFmpeg()),
		Proxy:     proxy,
		LimitKBps: opt.SpeedLimit,
	}
	tr := &ytdlp.Tracker{Estimate: job.GetTotalSize()}
	final, err := ytdlp.Run(ctx, spec, func(p ytdlp.Progress) {
		done, total := tr.Feed(p)
		job.SetDownloaded(done)
		if total > job.GetTotalSize() {
			job.SetTotalSize(total)
		}
		progressFn(done, job.GetTotalSize())
	})
	if err != nil {
		return err
	}
	size := job.GetTotalSize()
	if st, serr := os.Stat(final); serr == nil {
		size = st.Size()
	}
	job.SetOutputPath(final)
	job.SetFileName(filepath.Base(final))
	job.SetTotalSize(size)
	job.SetDownloaded(size)
	progressFn(size, size)
	return nil
}

// removeJobPartials discards a job's unfinished data. For a video that is every
// yt-dlp work file next to the planned name; for a file download, the usual
// partial + sidecar. A finished video file is never touched.
func removeJobPartials(job *Job, path string) {
	if job != nil && job.IsVideo() {
		ytdlp.CleanPartials(filepath.Dir(path), filepath.Base(path))
		return
	}
	RemovePartialData(path)
}

func errVideoUnsupported(what string) error {
	return &CodedError{Code: codeUnsupported, Kind: KindUnsupported, Err: fmt.Errorf("%s is not available for video downloads", what)}
}
