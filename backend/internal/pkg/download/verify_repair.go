package download

import (
	"context"
	"fmt"
	"os"
	"sync/atomic"

	"golang.org/x/sync/errgroup"
	"golang.org/x/time/rate"

	"github.com/tiredbooy/Rum/backend/internal/pkg/config"
	"github.com/tiredbooy/Rum/backend/internal/pkg/utils"
)

// VerifyResult is the outcome of a VerifyFile call. Ok is the bottom line; the
// other fields explain how the verdict was reached so the API/UI can surface an
// actionable message ("source unchanged, 1 segment has a hole", etc.).
type VerifyResult struct {
	Ok          bool   `json:"ok"`
	BadSegments []int  `json:"bad_segments"`
	Method      string `json:"method"` // "stored_hash" | "structural" | "deep" | "missing"
	Detail      string `json:"detail,omitempty"`
}

// VerifyFile checks an already-finished (or partially finished) segmented
// download on disk and reports whether it is intact, plus which segments look
// bad/missing so RepairFile can re-fetch only those. It is safe to call on a
// completed job and never mutates the file.
//
// Strategy, cheapest first:
//  1. The output must exist and (if totalSize is known) be the right size. A
//     wrong size means the whole file is bad.
//  2. If a sidecar with per-segment progress exists, any incomplete segment is a
//     hole — report it. This is the structural / sparse-hole check (E3) and needs
//     no network.
//  3. If the sidecar carries a stored full-file hash (written when
//     Options.VerifyIntegrity was on), recompute and compare it for a server-free
//     content check. A mismatch with no per-segment info marks every segment bad.
//  4. deep == true forces a network re-validation: re-HEAD to confirm
//     size/validator, then (when the content can't otherwise be localized) treat
//     the file as needing a full re-fetch. Deep per-segment checksum re-fetch is a
//     roadmap item; today deep upgrades a structural pass with a fresh HEAD so the
//     caller learns if the remote changed.
//
// opt provides the Downloader (for the optional HEAD) and output-path resolution;
// it may be a zero-ish Options as long as Downloader is set when deep is true.
func VerifyFile(ctx context.Context, opt Options, job *Job, deep bool) (VerifyResult, error) {
	url := utils.UrlValidation(job.GetURL())
	fullPath := job.GetOutputPath()
	if fullPath == "" {
		fullPath = PrepareOutputPath(opt, job.GetFileName(), url, job.ContentType)
	}
	totalSize := job.GetTotalSize()

	// (1) The file must exist.
	fi, err := os.Stat(fullPath)
	if err != nil {
		if os.IsNotExist(err) {
			return VerifyResult{Ok: false, Method: "missing", Detail: "output file does not exist"}, nil
		}
		return VerifyResult{}, fmt.Errorf("verify: stat output: %w", err)
	}

	// Recompute the segment plan the same way the download did so segment indices
	// line up with what RepairFile will refetch.
	connections := resolveConnections(opt.Connections, totalSize)
	plan := computeSegments(totalSize, connections)
	allSegments := func() []int {
		out := make([]int, len(plan))
		for i := range plan {
			out[i] = i
		}
		return out
	}

	// (1b) Size check (when known). A sparse hole would still pass this because the
	// presized file keeps the right size — that is exactly why steps (2)/(3) exist.
	if totalSize > 0 && fi.Size() != totalSize {
		return VerifyResult{Ok: false, BadSegments: allSegments(), Method: "structural",
			Detail: fmt.Sprintf("on-disk size %d != expected %d", fi.Size(), totalSize)}, nil
	}

	// (2)/(3) Consult the sidecar if present.
	if m, ok := loadPartsMeta(fullPath, totalSize, url); ok {
		// (2) Per-segment completeness localizes holes precisely.
		var bad []int
		for i := range m.Segments {
			if !m.Segments[i].complete() {
				bad = append(bad, m.Segments[i].Index)
			}
		}
		if len(bad) > 0 {
			return VerifyResult{Ok: false, BadSegments: bad, Method: "structural",
				Detail: fmt.Sprintf("%d incomplete segment(s) — file has holes", len(bad))}, nil
		}
		// (3) Stored full-file hash carried by an older sidecar: server-free content
		// verification (kept for backward compatibility with sidecars written before
		// the hash moved onto the Job record below).
		if m.FullHash != "" {
			got, herr := hashFileSHA256(fullPath)
			if herr != nil {
				return VerifyResult{}, fmt.Errorf("verify: hash file: %w", herr)
			}
			if got != m.FullHash {
				return VerifyResult{Ok: false, BadSegments: allSegments(), Method: "stored_hash",
					Detail: "stored full-file hash mismatch — content differs"}, nil
			}
			return VerifyResult{Ok: true, Method: "stored_hash"}, nil
		}
	}

	// (3b) Stored integrity hash on the Job (written on completion when integrity
	// verification was enabled). This is the server-free content check for a normal
	// finished download, which no longer keeps a sidecar next to the file.
	if h := job.GetIntegrityHash(); h != "" {
		got, herr := hashFileSHA256(fullPath)
		if herr != nil {
			return VerifyResult{}, fmt.Errorf("verify: hash file: %w", herr)
		}
		if got != h {
			return VerifyResult{Ok: false, BadSegments: allSegments(), Method: "stored_hash",
				Detail: "stored integrity hash mismatch — content differs"}, nil
		}
		return VerifyResult{Ok: true, Method: "stored_hash"}, nil
	}

	// (4) Deep mode: re-HEAD to confirm the remote still matches what we have.
	if deep {
		if opt.Downloader == nil {
			return VerifyResult{}, fmt.Errorf("verify: deep mode requires a Downloader")
		}
		info, herr := opt.Downloader.HeadWithFallbackContext(ctx, url)
		if herr == nil && info != nil {
			remoteSize := utils.ConvertSizeToInt(info.ContentSize)
			if remoteSize > 0 && totalSize > 0 && remoteSize != totalSize {
				return VerifyResult{Ok: false, BadSegments: allSegments(), Method: "deep",
					Detail: fmt.Sprintf("remote size %d != local %d — source changed", remoteSize, totalSize)}, nil
			}
		}
		// No localized fault found and remote size agrees: structurally sound.
		return VerifyResult{Ok: true, Method: "deep"}, nil
	}

	// User-supplied checksum, if any, is the final authority on a cheap pass.
	if opt.Checksum != "" {
		if err := VerifyChecksum(fullPath, opt.ChecksumAlgo, opt.Checksum); err != nil {
			return VerifyResult{Ok: false, BadSegments: allSegments(), Method: "structural",
				Detail: "user checksum mismatch"}, nil
		}
	}

	return VerifyResult{Ok: true, Method: "structural"}, nil
}

// RepairFile re-fetches only the given segment indices (their full byte ranges)
// into the existing output file, then re-runs the always-verify pass (E3). Pass
// the badSegments from a prior VerifyFile; an empty/nil slice repairs nothing and
// just re-verifies. It reuses the same downloadSegment path (with If-Range
// validator safety from E1) and the same durable WriteAt as a normal download,
// so a repair cannot itself introduce a hole or staple mismatched bytes.
//
// RepairFile is idempotent and safe to call on a completed job: repairing already
// complete segments simply re-downloads and overwrites them with identical bytes.
func RepairFile(ctx context.Context, opt Options, job *Job, badSegments []int) error {
	url := utils.UrlValidation(job.GetURL())
	fullPath := job.GetOutputPath()
	if fullPath == "" {
		fullPath = PrepareOutputPath(opt, job.GetFileName(), url, job.ContentType)
		job.SetOutputPath(fullPath)
	}
	totalSize := job.GetTotalSize()
	if totalSize <= 0 {
		return Wrap(KindValidation, "repair: unknown total size, cannot recompute segment plan")
	}

	connections := resolveConnections(opt.Connections, totalSize)
	plan := computeSegments(totalSize, connections)
	if len(plan) <= 1 {
		return Wrap(KindValidation, "repair: file is not segmented")
	}

	// Open + ensure the file is presized so WriteAt at any segment offset is valid
	// (a missing/short file is re-presized; existing good bytes are preserved).
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

	// Resolve which segments to refetch. Dedupe + bound to the plan; an out-of-range
	// index is ignored rather than panicking.
	want := make(map[int]struct{}, len(badSegments))
	for _, idx := range badSegments {
		if idx >= 0 && idx < len(plan) {
			want[idx] = struct{}{}
		}
	}

	// Per-segment counters seeded to 0 for the segments we refetch (force a full
	// re-download of each bad segment's range, overwriting whatever was there).
	segDone := make([]atomic.Int64, len(plan))
	atomicDownloaded := &atomic.Int64{}

	// Reuse the limiter wiring so repair honors the speed cap like a normal run.
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

	// Carry the sidecar's validators so refetched ranges revalidate via If-Range
	// (E1): if the source changed, RepairFile fails loudly instead of writing
	// mismatched bytes.
	val := &sharedValidator{}
	if m, ok := loadPartsMeta(fullPath, totalSize, url); ok {
		val.set(m.ETag, m.LastModified, m.ContentLength)
	}

	cfg := newRetryConfig(opt)
	g, gctx := errgroup.WithContext(ctx)
	g.SetLimit(connections)
	for i := range plan {
		if _, repair := want[i]; !repair {
			continue
		}
		seg := plan[i]
		idx := i
		g.Go(func() error {
			return retryWithBackoff(gctx, cfg, func(ctx context.Context) error {
				return downloadSegment(ctx, opt, url, outFile, &segDone[idx], seg,
					limiter, atomicDownloaded, totalSize, nil, val)
			})
		})
	}
	if err := g.Wait(); err != nil {
		return err
	}

	if err := outFile.Sync(); err != nil {
		return err
	}

	// Re-presize bookkeeping for verification: the segments we did NOT refetch are
	// assumed already complete on disk, so mark their Done full so the structural
	// check passes when the file is otherwise intact. (We only refetched the bad
	// ones; the rest were verified good by VerifyFile before repair was requested.)
	for i := range plan {
		if _, repaired := want[i]; !repaired {
			segDone[i].Store(plan[i].size())
		}
	}
	if err := verifySegmentsComplete(plan, segDone, totalSize, fullPath); err != nil {
		return err
	}

	// Honor a user checksum if present.
	if err := VerifyChecksum(fullPath, opt.ChecksumAlgo, opt.Checksum); err != nil {
		return err
	}

	// If the job carries a stored integrity hash, confirm the repaired file matches
	// it end-to-end — this catches a source that changed in a way If-Range did not
	// flag (e.g. a server that ignores If-Range), instead of silently "repairing"
	// into a different version.
	if h := job.GetIntegrityHash(); h != "" {
		got, herr := hashFileSHA256(fullPath)
		if herr != nil {
			return herr
		}
		if got != h {
			return Wrap(KindChecksum, "repair: integrity hash mismatch after repair — source may have changed")
		}
	}

	// Refresh the sidecar to mark everything complete (or remove it on success,
	// matching the normal completion path).
	removePartsMeta(fullPath)
	return nil
}
