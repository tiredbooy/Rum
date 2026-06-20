package download

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
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
	// (Connections <= 0 means "single stream", so callers must set it). 8 mirrors
	// IDM's default and roughly doubles throughput against CDNs that rate-limit
	// each connection (the common case for game/file-host CDNs); see the
	// user-tunable "connections" setting (clamped to maxConnections).
	defaultConnections = 8
	// maxConnections caps how many segments we will ever open for one file. It is
	// also the upper bound of the user-facing connections setting/slider.
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

// partsMetaVersion is the on-disk schema version of the sidecar. Bumped from the
// implicit v0 (no field) to v1 when ETag/LastModified/ContentLength/FullHash were
// added for validator-aware resume (E1) and server-free verification (E3/E4).
const partsMetaVersion = 1

type partsMeta struct {
	TotalSize int64     `json:"total_size"`
	URL       string    `json:"url"`
	Segments  []segment `json:"segments"`

	// Version is the sidecar schema version. Older sidecars (pre-validator) have
	// Version == 0 and simply carry no validators; resume still works for them via
	// the size-only fallback in ValidateResume.
	Version int `json:"version,omitempty"`

	// HTTP validators captured from the first segment's 206 response when the
	// download began. On resume these are sent back as If-Range so the server can
	// tell us (via 206 vs 200) whether the remote bytes are still the same ones we
	// already have on disk. Empty means the server supplied no validator and resume
	// is best-effort, guarded by the always-verify pass (E3). See C1 in the design
	// doc: without this, resume could staple new-version bytes onto old ones.
	ETag         string `json:"etag,omitempty"`
	LastModified string `json:"last_modified,omitempty"`
	// ContentLength is the total size the server reported (via Content-Range) when
	// the download began. Persisted so a future verify can re-confirm the size
	// without re-deriving it, and as a second consistency signal alongside TotalSize.
	ContentLength int64 `json:"content_length,omitempty"`

	// FullHash, when set, is the hex SHA-256 of the fully assembled file, computed
	// on completion when Options.VerifyIntegrity is on. It lets a later VerifyFile
	// re-verify the bytes on disk without contacting the server. FullHashAlgo names
	// the algorithm (currently always "sha256").
	FullHash     string `json:"full_hash,omitempty"`
	FullHashAlgo string `json:"full_hash_algo,omitempty"`
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

// savePartsMeta durably persists the sidecar. The previous implementation wrote
// a tmp file and renamed it but never fsync'd anything, so after a hard kill /
// power-loss the rename (and even the tmp's bytes) could be lost while the page
// cache still claimed progress — exactly C2 in the design doc. We now: write the
// tmp, f.Sync() its bytes, rename atomically, then fsync the PARENT DIRECTORY so
// the rename itself is durable. The caller is responsible for syncing the data
// file BEFORE calling this so we never record progress not yet on disk.
func savePartsMeta(outputPath string, m *partsMeta) {
	data, err := json.Marshal(m)
	if err != nil {
		return
	}
	tmp := partsFilePath(outputPath) + ".tmp"
	f, err := os.OpenFile(tmp, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0644)
	if err != nil {
		return
	}
	if _, err := f.Write(data); err != nil {
		_ = f.Close()
		_ = os.Remove(tmp)
		return
	}
	// fsync the sidecar bytes before the rename so the rename can't expose a
	// truncated/empty sidecar after a crash.
	if err := f.Sync(); err != nil {
		_ = f.Close()
		_ = os.Remove(tmp)
		return
	}
	if err := f.Close(); err != nil {
		_ = os.Remove(tmp)
		return
	}
	if err := os.Rename(tmp, partsFilePath(outputPath)); err != nil {
		_ = os.Remove(tmp)
		return
	}
	// fsync the parent directory so the rename (a directory metadata change) is
	// itself durable; without this the file can revert to its pre-rename state
	// after a power-loss even though its data was fsync'd.
	fsyncDir(filepath.Dir(outputPath))
}

// fsyncDir flushes a directory's metadata so a rename/create inside it survives a
// crash. It is best-effort: directories cannot be fsync'd on every platform
// (notably Windows returns an error), and a failure here only weakens durability,
// it does not corrupt anything — so errors are swallowed.
func fsyncDir(dir string) {
	d, err := os.Open(dir)
	if err != nil {
		return
	}
	defer d.Close()
	_ = d.Sync()
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
	finalPath := PrepareOutputPath(opt, job.FileName, url, job.ContentType)

	// When a TempDir is configured, the in-progress file + its resume sidecar live
	// under TempDir and are moved to finalPath on completion. The working path is
	// what we download into and what the job/sidecar point at during the transfer;
	// finalizeWorkingMove swaps it back to finalPath on success.
	fullPath := resolveWorkingPath(opt, finalPath)
	job.SetOutputPath(fullPath)

	totalSize := job.GetTotalSize()
	connections := resolveConnections(opt.Connections, totalSize)

	if !shouldSegment(job.SupportRange, totalSize, opt.Connections) {
		return DownloadSingleFile(ctx, opt, job, progressFn)
	}

	job.SetTotalSize(totalSize)

	// Fail fast if the destination filesystem clearly cannot hold the file,
	// instead of allocating, downloading partway, and dying with ENOSPC. Degrades
	// to a no-op on platforms where free space can't be determined.
	if err := PreflightDiskSpace(filepath.Dir(fullPath), totalSize); err != nil {
		return err
	}

	// A segmented download may need to restart from zero exactly once when the
	// remote validator changed under us (E1): the server answers our resume
	// If-Range with a 200 full body instead of a 206, meaning the bytes we already
	// have belong to a different version. Restarting is a fresh pass with a clean
	// plan, so we loop instead of recursing. The bool is "please restart".
	for {
		restart, err := runSegmentedPass(ctx, opt, job, progressFn, url, fullPath, finalPath, totalSize, connections)
		if restart {
			// Source changed: discard stale resume state and start over. The sidecar
			// + on-disk holes have already been cleared by the pass before it asked
			// for a restart, so the next iteration computes a virgin plan.
			DebugLog("segmented: source changed under resume, restarting from zero")
			continue
		}
		return err
	}
}

// runSegmentedPass performs one full segmented-download attempt against fullPath.
// It returns restart=true (with a nil error) when the remote validator changed
// mid-resume and the caller should start over from a clean state; otherwise it
// returns the terminal error (nil on success, ctx error on pause/cancel, or a
// real failure). Splitting this out of DownloadSegmented makes the "restart from
// zero" path (E1) a plain loop iteration rather than recursion, and keeps the
// durability finalizer (E2) scoped to a single pass's file handle + goroutine.
func runSegmentedPass(ctx context.Context, opt Options, job *Job, progressFn ProgressFunc, url, fullPath, finalPath string, totalSize int64, connections int) (restart bool, retErr error) {
	// Build / restore segment plan and any saved validators from the sidecar.
	var segs []segment
	var saved *partsMeta
	if m, ok := loadPartsMeta(fullPath, totalSize, url); ok {
		segs = m.Segments
		saved = m
		DebugLog("Resuming segmented download from sidecar parts file")
	} else {
		segs = computeSegments(totalSize, connections)
	}
	if len(segs) <= 1 {
		return false, DownloadSingleFile(ctx, opt, job, progressFn)
	}

	// Open the output file and pre-size it so WriteAt at any offset is valid.
	outFile, err := os.OpenFile(fullPath, os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return false, err
	}
	defer outFile.Close()

	if fi, statErr := outFile.Stat(); statErr != nil || fi.Size() != totalSize {
		if err := outFile.Truncate(totalSize); err != nil {
			return false, err
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

	// Shared validator state (E1). Captured atomically from the first segment's
	// 206 response (its ETag/Last-Modified/total-from-Content-Range) so it can be
	// persisted in every snapshot and revalidated on the next resume. We seed it
	// from any sidecar we just loaded so a resume keeps the original validators
	// even if the server stops echoing them. The mutex guards first-writer-wins.
	val := &sharedValidator{}
	if saved != nil {
		val.set(saved.ETag, saved.LastModified, saved.ContentLength)
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

	// buildSnapshot reads the live per-segment counters + captured validators into
	// a fresh sidecar struct. It is called both by the periodic goroutine and the
	// final finalizer, so it must be safe to call concurrently with the workers
	// (it only Loads atomics and reads the immutable plan / once-written validators).
	buildSnapshot := func() *partsMeta {
		etag, lm, clen := val.get()
		m := &partsMeta{
			TotalSize:     totalSize,
			URL:           url,
			Version:       partsMetaVersion,
			ETag:          etag,
			LastModified:  lm,
			ContentLength: clen,
			Segments:      make([]segment, len(segs)),
		}
		for i := range segs {
			m.Segments[i] = segment{
				Index: segs[i].Index,
				Start: segs[i].Start,
				End:   segs[i].End,
				Done:  segDone[i].Load(),
			}
		}
		return m
	}

	// snapshot enforces the durability invariant "never record progress you
	// haven't made durable" (E2): fsync the file DATA first, THEN write the
	// sidecar (which is itself fsync'd + dir-fsync'd in savePartsMeta). If the
	// data sync fails we skip the sidecar write rather than persist progress that
	// is not on disk.
	snapshot := func() {
		if syncErr := outFile.Sync(); syncErr != nil {
			DebugLog("segmented: data sync before snapshot failed: " + syncErr.Error())
			return
		}
		savePartsMeta(fullPath, buildSnapshot())
	}

	// Periodic persistence of segment progress so a crash/pause can resume.
	persistCtx, stopPersist := context.WithCancel(ctx)
	persistDone := make(chan struct{})
	go func() {
		defer close(persistDone)
		ticker := time.NewTicker(2 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-persistCtx.Done():
				// Final durable flush on pause/cancel/stop. This runs BEFORE
				// runSegmentedPass returns (we block on persistDone below), so a
				// paused download's tail is fsync'd and its sidecar is current —
				// fixing C2 (pause previously skipped the sync via an early return).
				snapshot()
				return
			case <-ticker.C:
				snapshot()
			}
		}
	}()

	cfg := newRetryConfig(opt)

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
					limiter, atomicDownloaded, totalSize, safeProgress, val)
			})
		})
	}

	err = g.Wait()

	// Stop persistence and block until the final durable snapshot has been
	// written. We do this for EVERY exit path (success, pause, error) so we never
	// lose progress that is already on disk.
	stopPersist()
	<-persistDone

	// E1: the remote validator changed under us. A worker detected the server
	// answering our If-Range resume with a 200 (full body) instead of a 206.
	// Stapling that new-version body onto old-version bytes is exactly the C1
	// corruption, so instead we discard ALL resume state and ask the caller to
	// restart from zero with a clean plan. Truncate to drop the stale bytes; the
	// next pass re-presizes and refetches everything.
	if errors.Is(err, errValidatorChanged) {
		removePartsMeta(fullPath)
		if tErr := outFile.Truncate(0); tErr != nil {
			return false, tErr
		}
		_ = outFile.Sync()
		return true, nil
	}

	if err != nil {
		// Pause/cancel is not a failure: the final snapshot above already flushed a
		// resumable sidecar, so just surface the context error. A real error keeps
		// the sidecar too (we never delete partial data on failure — E3) so a later
		// retry/repair can use it.
		return false, err
	}

	if syncErr := outFile.Sync(); syncErr != nil {
		return false, syncErr
	}

	// Always-verify on completion (E3): even with no user checksum, confirm every
	// segment is complete, the segment sizes sum to totalSize, and the on-disk size
	// equals totalSize. The on-disk size check catches sparse holes that Truncate
	// leaves (a hole reads back as zeros and would pass a naive size compare). On
	// failure we KEEP the sidecar and partial file so the user can verify/repair.
	if err := verifySegmentsComplete(segs, segDone, totalSize, fullPath); err != nil {
		return false, err
	}

	// Verify a user-supplied checksum if present (unchanged behavior, kept on top
	// of the structural check above). A mismatch keeps the sidecar for retry.
	if err := VerifyChecksum(fullPath, opt.ChecksumAlgo, opt.Checksum); err != nil {
		return false, err
	}

	// Optional server-free integrity record (E3/E4): when enabled, compute the
	// full-file SHA-256 and stash it ON THE JOB (persisted in jobs.json), so a
	// future VerifyFile can re-check the on-disk bytes without the server — without
	// leaving a visible *.rumparts sidecar next to every finished download. The
	// sidecar is still removed below like any normal success, so a completed
	// download never litters the user's folder with resume metadata.
	if opt.VerifyIntegrity {
		hash, herr := hashFileSHA256(fullPath)
		if herr != nil {
			return false, herr
		}
		job.SetIntegrityHash(hash, "sha256")
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

	// Move the finished file out of TempDir to its real destination (no-op when
	// TempDir is unset) before auto-organizing.
	if err := finalizeWorkingMove(opt, job, fullPath, finalPath); err != nil {
		return false, err
	}

	// Auto-organize into a category directory (no-op unless opt.Categorize and a
	// matching rule). Never lose a finished file on a categorize error.
	if err := finalizeCategorize(opt, job); err != nil {
		DebugLog("categorize failed: " + err.Error())
	}

	return false, nil
}

// errValidatorChanged is an internal sentinel: a worker saw the server answer our
// If-Range resume with a 200 (full body) instead of a 206, meaning the remote
// resource changed. It is caught by runSegmentedPass to trigger a clean restart
// from zero (E1); it must NOT be retried (resuming would only re-corrupt) so it
// is reported non-retryable.
var errValidatorChanged = errors.New("remote validator changed during resume")

// sharedValidator captures the resource validators (ETag / Last-Modified) and the
// server-reported total (from the first 206's Content-Range) once, on a
// first-writer-wins basis, so all segments agree on the same validators and the
// persistence goroutine can snapshot them without racing the workers.
type sharedValidator struct {
	mu           sync.Mutex
	set_         bool
	etag         string
	lastModified string
	contentLen   int64
}

// set records the validators the first time it is called with a non-empty
// signal; later calls are ignored (first-writer-wins). Called from the first
// segment that receives a 206. Recording even empty validators (set_=true) marks
// that we have observed the server's answer.
func (s *sharedValidator) set(etag, lastMod string, contentLen int64) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.set_ {
		return
	}
	s.set_ = true
	s.etag = etag
	s.lastModified = lastMod
	s.contentLen = contentLen
}

// get returns the captured validators (zero values if none captured yet).
func (s *sharedValidator) get() (etag, lastMod string, contentLen int64) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.etag, s.lastModified, s.contentLen
}

// has reports whether a usable validator (ETag or Last-Modified) was captured.
func (s *sharedValidator) has() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.etag != "" || s.lastModified != ""
}

// downloadSegment fetches the still-missing portion of one segment and writes it
// at the correct file offset via WriteAt. It updates segDone (this segment's
// progress) and atomicDownloaded (the whole-file aggregate) as it goes.
//
// val carries cross-segment validator state (E1): if validators were already
// captured (from the sidecar on resume, or by an earlier segment this session)
// we send them back as If-Range so the server confirms — via 206 vs 200 — that
// the bytes we hold are still the same version. The first 206 we see records the
// server's validators into val for persistence and for sibling segments.
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
	val *sharedValidator,
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
	// E1: send If-Range with whatever validator we already trust (preferring the
	// strong ETag). Per RFC 7233 the server returns 206 if it still matches, or a
	// 200 full body if the resource changed. Sending this only matters when we are
	// resuming onto bytes we already have (done > 0 or a sidecar validator exists);
	// a fresh segment has nothing to protect but it is harmless to include.
	if val != nil {
		if etag, lm, _ := val.get(); etag != "" {
			req.Header.Set("If-Range", etag)
		} else if lm != "" {
			req.Header.Set("If-Range", lm)
		}
	}
	req = req.WithContext(ctx)

	resp, err := opt.Downloader.Client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusPartialContent {
		// A 200 means the server sent the WHOLE file instead of our requested range.
		// Two cases, distinguished for correctness (E1):
		//   - We sent If-Range and have prior bytes/validators: the validator no
		//     longer matches, so the server legitimately returned the full new
		//     version. Surface errValidatorChanged so the caller restarts cleanly
		//     instead of stapling new bytes at this segment's offset (C1 corruption).
		//   - Otherwise the server simply ignored our Range (it claimed range support
		//     but doesn't honor it): still unsafe to write a full body into one
		//     segment, so error out as before.
		if resp.StatusCode == http.StatusOK {
			if val != nil && (val.has() || done > 0) {
				return errValidatorChanged
			}
			return fmt.Errorf("server ignored Range request for segment %d (got 200)", seg.Index)
		}
		return &httpStatusError{code: resp.StatusCode}
	}

	// First 206 wins the right to record the resource validators for the whole
	// download (used for the next resume's If-Range and for verification). We pull
	// the total from Content-Range ("bytes start-end/total") when present.
	if val != nil {
		val.set(resp.Header.Get("ETag"), resp.Header.Get("Last-Modified"), totalFromContentRange(resp.Header.Get("Content-Range")))
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
			// Defensive (E2): WriteAt must write the full slice or return an error
			// per io.WriterAt; we still assert n to belt-and-suspenders against any
			// wrapper that violates the contract, so we never advance our counters
			// past bytes that did not land on disk.
			wn, werr := outFile.WriteAt(buf[:n], offset)
			if werr != nil {
				return werr
			}
			if wn != n {
				return fmt.Errorf("short write for segment %d: wrote %d of %d bytes", seg.Index, wn, n)
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

// totalFromContentRange extracts the total resource size from an HTTP
// Content-Range header value ("bytes 0-99/12345" -> 12345). Returns 0 when the
// header is absent, malformed, or the total is unknown ("*").
func totalFromContentRange(cr string) int64 {
	if cr == "" {
		return 0
	}
	idx := strings.LastIndex(cr, "/")
	if idx == -1 {
		return 0
	}
	total := strings.TrimSpace(cr[idx+1:])
	if total == "" || total == "*" {
		return 0
	}
	n, err := strconv.ParseInt(total, 10, 64)
	if err != nil {
		return 0
	}
	return n
}

// verifySegmentsComplete is the always-verify structural check (E3). It confirms
// (1) every segment is complete(), (2) the segment sizes sum to totalSize, and
// (3) the on-disk file size equals totalSize. The on-disk check is what catches a
// sparse hole: a missing region from a crash-truncated file reads back as zeros
// and would pass a size-only check, but here a hole shows up as an incomplete
// segment. Returns a non-nil, actionable error on any discrepancy; the caller
// keeps the sidecar so the file can be repaired.
func verifySegmentsComplete(segs []segment, segDone []atomic.Int64, totalSize int64, fullPath string) error {
	var sum int64
	for i := range segs {
		done := segDone[i].Load()
		s := segs[i]
		s.Done = done
		sum += s.size()
		if !s.complete() {
			return Wrap(KindChecksum, "integrity: segment %d incomplete (%d of %d bytes) — file may have a hole; verify or repair",
				s.Index, done, s.size())
		}
	}
	if sum != totalSize {
		return Wrap(KindChecksum, "integrity: segment sizes sum to %d, want %d", sum, totalSize)
	}
	fi, err := os.Stat(fullPath)
	if err != nil {
		return fmt.Errorf("integrity: stat output: %w", err)
	}
	if fi.Size() != totalSize {
		return Wrap(KindChecksum, "integrity: on-disk size %d != expected %d (possible sparse hole)", fi.Size(), totalSize)
	}
	return nil
}

// hashFileSHA256 streams the file at path through SHA-256 and returns the hex
// digest. Used for the optional server-free integrity record (E3/E4).
func hashFileSHA256(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}
