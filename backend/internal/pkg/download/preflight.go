package download

import (
	"fmt"
)

// diskSpaceSafetyMargin is added on top of the requested byte count when
// preflighting free space. Filesystems reserve some blocks, metadata grows the
// on-disk footprint slightly beyond the payload, and segmented downloads write a
// sidecar parts file. A small fixed margin avoids declaring "enough space" only
// to fail on the last few writes. We use the larger of a flat 16 MiB or ~1% of
// the requested size.
const diskSpaceFlatMargin = 16 * 1024 * 1024 // 16 MiB

// requiredWithMargin returns needBytes plus a safety margin (see
// diskSpaceFlatMargin). It is overflow-safe: if the addition would overflow it
// clamps to the maximum int64.
func requiredWithMargin(needBytes int64) int64 {
	if needBytes <= 0 {
		return diskSpaceFlatMargin
	}
	margin := int64(diskSpaceFlatMargin)
	if onePct := needBytes / 100; onePct > margin {
		margin = onePct
	}
	// Overflow guard: needBytes + margin must not wrap negative.
	if needBytes > (1<<63-1)-margin {
		return 1<<63 - 1
	}
	return needBytes + margin
}

// PreflightDiskSpace checks that the filesystem holding dir has enough free
// space to hold needBytes (plus a safety margin) before a download begins
// allocating/truncating the output file. It is intended to be called by the
// segmented finalize/allocation path so we fail fast with a clear, classified
// error instead of running out of space mid-write.
//
// Behavior:
//   - needBytes <= 0 means the remote size is unknown; we cannot meaningfully
//     compare against free space, so this is a no-op (returns nil).
//   - On platforms where free space cannot be determined (availableDiskBytes
//     returns ok == false), we degrade gracefully and return nil rather than
//     blocking a download we cannot prove will fail.
//   - When free space is known and insufficient, it returns an error wrapping
//     ErrInsufficientDiskSpace (owned by the errors taxonomy) with human detail.
func PreflightDiskSpace(dir string, needBytes int64) error {
	if needBytes <= 0 {
		// Unknown size: nothing to check.
		return nil
	}

	avail, ok := availableDiskBytes(dir)
	if !ok {
		// Could not determine free space on this platform/path: do not block.
		DebugLog("PreflightDiskSpace: free space undeterminable for " + dir + ", skipping check")
		return nil
	}

	need := requiredWithMargin(needBytes)
	if avail < need {
		return fmt.Errorf("preflight: need %d bytes (incl. margin) but only %d available in %q: %w",
			need, avail, dir, ErrInsufficientDiskSpace)
	}
	return nil
}
