package download

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
	"time"

	"golang.org/x/sync/errgroup"
	"golang.org/x/time/rate"

	"github.com/tiredbooy/Rum/backend/internal/pkg/config"
	"github.com/tiredbooy/Rum/backend/internal/pkg/utils"
)

const (
	// defaultConnections is used when a job opts into segmented downloading
	// (Connections <= 0 means "single stream", so callers must set it).
	defaultConnections = 4
	// maxConnections caps how many segments we will ever open for one file.
	maxConnections = 16
	// minSegmentedSize is the smallest total size for which segmenting is worth
	// the overhead of multiple connections + a sidecar file (4 MiB).
	minSegmentedSize = 4 * 1024 * 1024
	// minSegmentBytes ensures each segment is reasonably large; tiny segments
	// waste connections. We reduce the connection count to honor this.
	minSegmentBytes = 1024 * 1024 // 1 MiB
)

// segment is an inclusive byte range [Start, End] of the output file plus how
// many of its bytes have already been written (for resume).
type segment struct {
	Index int   `json:"index"`
	Start int64 `json:"start"`
	End   int64 `json:"end"` // inclusive
	Done  int64 `json:"done"`
}

// size returns the total number of bytes in the segment.
func (s segment) size() int64 { return s.End - s.Start + 1 }

// remaining returns how many bytes of the segment still need downloading.
func (s segment) remaining() int64 { return s.size() - s.Done }

// complete reports whether the segment has been fully written.
func (s segment) complete() bool { return s.Done >= s.size() }

// resolveConnections normalizes the requested connection count for a file of
// totalSize bytes, returning the number of segments to use. A return of 1 means
// "use the single-stream path".
func resolveConnections(requested int, totalSize int64) int {
	if requested <= 1 {
		return 1
	}
	if requested > maxConnections {
		requested = maxConnections
	}
	if totalSize <= 0 {
		return 1
	}
	// Don't create segments smaller than minSegmentBytes.
	maxBySize := int(totalSize / minSegmentBytes)
	if maxBySize < 1 {
		maxBySize = 1
	}
	if requested > maxBySize {
		requested = maxBySize
	}
	if requested < 1 {
		requested = 1
	}
	return requested
}

// shouldSegment decides whether a segmented download is applicable.
func shouldSegment(supportRange bool, totalSize int64, connections int) bool {
	if !supportRange {
		return false
	}
	if totalSize < minSegmentedSize {
		return false
	}
	return resolveConnections(connections, totalSize) > 1
}

// computeSegments splits a file of totalSize bytes into n contiguous,
// non-overlapping segments covering [0, totalSize). The last segment absorbs
// any remainder so the union is exactly the whole file. n is assumed already
// normalized via resolveConnections (n >= 1, n <= totalSize).
func computeSegments(totalSize int64, n int) []segment {
	if n < 1 {
		n = 1
	}
	if totalSize <= 0 {
		return nil
	}
	if int64(n) > totalSize {
		n = int(totalSize)
	}
	base := totalSize / int64(n)
	segs := make([]segment, 0, n)
	var start int64
	for i := 0; i < n; i++ {
		end := start + base - 1
		if i == n-1 {
			end = totalSize - 1 // last segment takes the remainder
		}
		segs = append(segs, segment{Index: i, Start: start, End: end})
		start = end + 1
	}
	return segs
}

// partsFilePath returns the sidecar metadata path used to persist per-segment
// progress for resume.
func partsFilePath(outputPath string) string {
	return outputPath + ".rumparts"
}

type partsMeta struct {
	TotalSize int64     `json:"total_size"`
	URL       string    `json:"url"`
	Segments  []segment `json:"segments"`
}

func loadPartsMeta(outputPath string, totalSize int64, url string) (*partsMeta, bool) {
	data, err := os.ReadFile(partsFilePath(outputPath))
	if err != nil {
		return nil, false
	}
	var m partsMeta
	if err := json.Unmarshal(data, &m); err != nil {
		return nil, false
	}
	// Only trust the sidecar if it matches this exact download.
	if m.TotalSize != totalSize || m.URL != url || len(m.Segments) == 0 {
		return nil, false
	}
	return &m, true
}

func savePartsMeta(outputPath string, m *partsMeta) {
	data, err := json.Marshal(m)
	if err != nil {
		return
	}
	tmp := partsFilePath(outputPath) + ".tmp"
	if err := os.WriteFile(tmp, data, 0644); err != nil {
		return
	}
	_ = os.Rename(tmp, partsFilePath(outputPath))
}

func removePartsMeta(outputPath string) {
	_ = os.Remove(partsFilePath(outputPath))
}

// DownloadSegmented downloads job using multiple concurrent range requests when
// applicable, falling back to DownloadSingleFile otherwise. Progress is
// aggregated across all segments into progressFn. The download is bounded
// (errgroup.SetLimit), fully context-cancellable (pause), and resumable via a
// sidecar parts file.
func DownloadSegmented(ctx context.Context, opt Options, job *Job, progressFn ProgressFunc) error {
	url := utils.UrlValidation(job.URL)
	fullPath := PrepareOutputPath(opt, job.FileName, url, job.ContentType)
	job.SetOutputPath(fullPath)

	totalSize := job.GetTotalSize()
	connections := resolveConnections(opt.Connections, totalSize)

	if !shouldSegment(job.SupportRange, totalSize, opt.Connections) {
		return DownloadSingleFile(ctx, opt, job, progressFn)
	}

	job.SetTotalSize(totalSize)

	// Build / restore segment plan.
	var segs []segment
	if m, ok := loadPartsMeta(fullPath, totalSize, url); ok {
		segs = m.Segments
		DebugLog("Resuming segmented download from sidecar parts file")
	} else {
		segs = computeSegments(totalSize, connections)
	}
	if len(segs) <= 1 {
		return DownloadSingleFile(ctx, opt, job, progressFn)
	}

	// Fail fast if the destination filesystem clearly cannot hold the file,
	// instead of allocating, downloading partway, and dying with ENOSPC. Degrades
	// to a no-op on platforms where free space can't be determined.
	if err := PreflightDiskSpace(filepath.Dir(fullPath), totalSize); err != nil {
		return err
	}

	// Open the output file and pre-size it so WriteAt at any offset is valid.
	outFile, err := os.OpenFile(fullPath, os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return err
	}
	defer outFile.Close()

	if fi, statErr := outFile.Stat(); statErr != nil || fi.Size() != totalSize {
		if err := outFile.Truncate(totalSize); err != nil {
			return err
		}
	}

	// Aggregate progress. Seed with already-downloaded bytes from the plan.
	var downloaded int64
	for i := range segs {
		downloaded += segs[i].Done
	}
	atomicDownloaded := &atomic.Int64{}
	atomicDownloaded.Store(downloaded)
	job.SetDownloaded(downloaded)

	// Segments run concurrently, so serialize progress callbacks behind a mutex.
	// This preserves the single-caller contract of ProgressFunc (the manager's
	// progressFn mutates unsynchronized speed/ETA state), so consumers do not
	// need to be goroutine-safe.
	var progressMu sync.Mutex
	var safeProgress ProgressFunc
	if progressFn != nil {
		safeProgress = func(d, total int64) {
			progressMu.Lock()
			progressFn(d, total)
			progressMu.Unlock()
		}
	}

	// Per-segment Done counters (atomic) so the persistence goroutine can read
	// progress without racing the workers.
	segDone := make([]atomic.Int64, len(segs))
	for i := range segs {
		segDone[i].Store(segs[i].Done)
	}

	// Speed limiter: shared across all segments so the global cap is honored.
	// When a governor is attached (desktop server path) the per-segment readers
	// use it instead (one live, process-wide budget); otherwise we build a single
	// per-download limiter shared across this download's segments.
	var limiter *rate.Limiter
	if opt.Governor == nil {
		var setting config.Setting
		_ = setting.LoadSettingMetadata()
		limitKB := setting.SpeedLimitKB
		if opt.SpeedLimit > 0 {
			limitKB = opt.SpeedLimit
		}
		limiter = newSpeedLimiter(limitKB)
	}

	// Periodic persistence of segment progress so a crash/pause can resume.
	persistCtx, stopPersist := context.WithCancel(ctx)
	persistDone := make(chan struct{})
	go func() {
		defer close(persistDone)
		ticker := time.NewTicker(2 * time.Second)
		defer ticker.Stop()
		snapshot := func() {
			m := &partsMeta{TotalSize: totalSize, URL: url, Segments: make([]segment, len(segs))}
			for i := range segs {
				m.Segments[i] = segment{
					Index: segs[i].Index,
					Start: segs[i].Start,
					End:   segs[i].End,
					Done:  segDone[i].Load(),
				}
			}
			savePartsMeta(fullPath, m)
		}
		for {
			select {
			case <-persistCtx.Done():
				snapshot()
				return
			case <-ticker.C:
				snapshot()
			}
		}
	}()

	cfg := newRetryConfig(opt.MaxRetries)

	g, gctx := errgroup.WithContext(ctx)
	g.SetLimit(connections)

	for i := range segs {
		seg := segs[i]
		idx := i
		if seg.complete() {
			continue
		}
		g.Go(func() error {
			return retryWithBackoff(gctx, cfg, func(ctx context.Context) error {
				return downloadSegment(ctx, opt, url, outFile, &segDone[idx], seg,
					limiter, atomicDownloaded, totalSize, safeProgress)
			})
		})
	}

	err = g.Wait()

	// Stop persistence and flush a final snapshot.
	stopPersist()
	<-persistDone

	if err != nil {
		return err
	}

	if syncErr := outFile.Sync(); syncErr != nil {
		return syncErr
	}

	// Verify integrity before we declare success and remove resume state. A
	// mismatch keeps the sidecar so the user can retry. No-op when no checksum
	// was supplied.
	if err := VerifyChecksum(fullPath, opt.ChecksumAlgo, opt.Checksum); err != nil {
		return err
	}

	// Success: drop the sidecar, finalize progress.
	removePartsMeta(fullPath)
	job.SetDownloaded(totalSize)
	if safeProgress != nil {
		safeProgress(totalSize, totalSize)
	}

	// Close the output file before any rename so auto-organize works on Windows
	// (an open file cannot be renamed there). The deferred Close becomes a no-op /
	// harmless double-close.
	_ = outFile.Close()

	// Auto-organize into a category directory (no-op unless opt.Categorize and a
	// matching rule). Never lose a finished file on a categorize error.
	if err := finalizeCategorize(opt, job); err != nil {
		DebugLog("categorize failed: " + err.Error())
	}

	return nil
}

// downloadSegment fetches the still-missing portion of one segment and writes it
// at the correct file offset via WriteAt. It updates segDone (this segment's
// progress) and atomicDownloaded (the whole-file aggregate) as it goes.
func downloadSegment(
	ctx context.Context,
	opt Options,
	url string,
	outFile *os.File,
	segDone *atomic.Int64,
	seg segment,
	limiter *rate.Limiter,
	atomicDownloaded *atomic.Int64,
	totalSize int64,
	progressFn ProgressFunc,
) error {
	// Recompute remaining from the live counter so retries resume correctly.
	done := segDone.Load()
	start := seg.Start + done
	if start > seg.End {
		return nil // already complete
	}

	req, err := opt.Downloader.NewRequest("GET", url)
	if err != nil {
		return err
	}
	req.Header.Set("Range", fmt.Sprintf("bytes=%d-%d", start, seg.End))
	req = req.WithContext(ctx)

	resp, err := opt.Downloader.Client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusPartialContent {
		// Anything other than 206 in a segmented context is unsafe: a 200 means
		// the server ignored our Range and would stream the WHOLE file into a
		// single segment's offset while sibling segments also write, corrupting
		// the result. shouldSegment already verified range support, so this is
		// a defensive guard. Surface it as a non-retryable error.
		if resp.StatusCode == http.StatusOK {
			return fmt.Errorf("server ignored Range request for segment %d (got 200)", seg.Index)
		}
		return &httpStatusError{code: resp.StatusCode}
	}

	var body io.Reader = resp.Body
	if opt.Governor != nil {
		// Shared global cap across all concurrent downloads/segments, honoring live
		// bandwidth-window changes.
		body = opt.Governor.Wrap(ctx, resp.Body)
	} else if limiter != nil {
		body = &rateLimitedReader{reader: resp.Body, limiter: limiter, ctx: ctx}
	}

	buf := make([]byte, downloadBufferSize)
	offset := start
	for {
		n, rerr := body.Read(buf)
		if n > 0 {
			if _, werr := outFile.WriteAt(buf[:n], offset); werr != nil {
				return werr
			}
			offset += int64(n)
			segDone.Add(int64(n))
			newTotal := atomicDownloaded.Add(int64(n))
			if progressFn != nil {
				progressFn(newTotal, totalSize)
			}
		}
		if rerr == io.EOF {
			return nil
		}
		if rerr != nil {
			if ctx.Err() != nil {
				return ctx.Err()
			}
			return rerr
		}
	}
}
