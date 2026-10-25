package download

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"time"

	"github.com/tiredbooy/Rum/backend/internal/pkg/utils"
)

// refreshProbeTimeout bounds the HEAD probe of a replacement link.
const refreshProbeTimeout = 30 * time.Second

// RefreshResult reports what RefreshJobURL did with the partial download.
type RefreshResult struct {
	// Resumed: the partial data was kept and the download continues from it.
	Resumed bool
	// Restarted: the partial data was discarded and the download starts from zero.
	Restarted bool
}

// readPartsMetaRaw reads the resume sidecar without requiring it to match a
// particular URL or size (loadPartsMeta does), for inspecting saved validators.
func readPartsMetaRaw(outputPath string) (*partsMeta, bool) {
	if outputPath == "" {
		return nil, false
	}
	data, err := os.ReadFile(partsFilePath(outputPath))
	if err != nil {
		return nil, false
	}
	var meta partsMeta
	if json.Unmarshal(data, &meta) != nil || len(meta.Segments) == 0 {
		return nil, false
	}
	return &meta, true
}

// RefreshJobURL swaps the URL of a failed or paused job for a fresh one (typically
// after a signed or time-limited link expired) and starts it again.
//
// The new link is probed first and checked against what the partial download was
// built from with ValidateResume (size and ETag / Last-Modified). On a match the
// partial data is kept and the download resumes from it. On a mismatch nothing is
// changed and an ErrRemoteChanged error is returned, so the caller can ask the
// user; calling again with restart=true discards the partial data and downloads
// the new link from zero.
func (m *JobManager) RefreshJobURL(ctx context.Context, id, newURL string, restart bool) (RefreshResult, error) {
	if err := ValidateURL(newURL); err != nil {
		return RefreshResult{}, err
	}

	m.mu.RLock()
	job, ok := m.jobs[id]
	m.mu.RUnlock()
	if !ok {
		return RefreshResult{}, fmt.Errorf("job %s not found: %w", id, ErrNotFound)
	}
	if s := job.GetStatus(); s != StatusError && s != StatusPaused {
		return RefreshResult{}, fmt.Errorf("job %s is %s; only failed or paused downloads can change link: %w", id, s, ErrConflict)
	}

	// Probe the new link with this job's own headers (Referer / User-Agent).
	opt := m.effectiveOptsForJob(job)
	probeCtx, cancel := context.WithTimeout(ctx, refreshProbeTimeout)
	defer cancel()
	info, err := opt.Downloader.HeadWithFallbackContext(probeCtx, newURL)
	if err != nil {
		return RefreshResult{}, fmt.Errorf("new link not reachable: %v: %w", redactErr(err), ErrInvalidURL)
	}
	if info.StatusCode < 200 || info.StatusCode >= 400 {
		if info.StatusCode == 0 {
			return RefreshResult{}, fmt.Errorf("new link not reachable: %w", ErrInvalidURL)
		}
		return RefreshResult{}, fmt.Errorf("new link not reachable (HTTP %d): %w", info.StatusCode, ErrInvalidURL)
	}
	remoteSize := utils.ConvertSizeToInt(info.ContentSize)

	// What the partial data was built from.
	outPath := job.GetOutputPath()
	savedSize := job.GetTotalSize()
	var savedETag, savedLastMod string
	meta, hasMeta := readPartsMetaRaw(outPath)
	if hasMeta {
		savedETag, savedLastMod = meta.ETag, meta.LastModified
		if savedSize <= 0 {
			savedSize = meta.TotalSize
		}
	}
	var onDisk int64
	if st, err := os.Stat(outPath); outPath != "" && err == nil {
		onDisk = st.Size()
	}
	hasPartial := hasMeta || onDisk > 0 || job.GetDownloaded() > 0

	// With nothing to protect any live link will do. Otherwise the new link must
	// still serve the same bytes AND honor Range, or the partial is useless.
	keep := !hasPartial ||
		(info.SupportsRange && ValidateResume(info.ETag, info.LastModified, savedETag, savedLastMod, remoteSize, savedSize))
	if !keep && !restart {
		return RefreshResult{}, fmt.Errorf("the new link is not the same file as the partial download: %w", ErrRemoteChanged)
	}

	m.mu.Lock()
	if s := job.GetStatus(); s != StatusError && s != StatusPaused {
		m.mu.Unlock()
		return RefreshResult{}, fmt.Errorf("job %s is %s; only failed or paused downloads can change link: %w", id, s, ErrConflict)
	}
	if other, taken := m.urls[newURL]; taken && other != id {
		m.mu.Unlock()
		return RefreshResult{}, fmt.Errorf("another download already uses this link: %w", ErrConflict)
	}
	switch {
	case !keep:
		RemovePartialData(outPath)
		job.SetDownloaded(0)
	case hasMeta:
		rewriteSidecarURL(outPath, newURL)
	}
	if !keep || savedSize <= 0 {
		job.SetTotalSize(remoteSize)
	}
	job.Mu.Lock()
	job.SupportRange = info.SupportsRange
	if info.ContentType != "" {
		job.ContentType = info.ContentType
	}
	job.Mu.Unlock()

	delete(m.urls, job.GetURL())
	m.urls[newURL] = id
	job.SetURL(newURL)
	job.SetError(nil)
	job.SetStatus(StatusPending)
	m.mu.Unlock()
	m.saveToDisk()

	if err := m.StartJob(ctx, id); err != nil {
		return RefreshResult{}, err
	}
	return RefreshResult{Resumed: keep, Restarted: !keep}, nil
}
