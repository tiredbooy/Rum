package download

import (
	"fmt"
	"os"
	"path/filepath"
)

// resolveWorkingPath returns the path the in-progress file (and its .rumparts
// resume sidecar) should be written to. When opt.TempDir is empty the working
// path IS the final path (legacy behavior: download straight into the output
// directory). When opt.TempDir is set, the in-progress file lives under TempDir
// using the same base filename, so the data and its sidecar stay co-located —
// which is what keeps resume correct across sessions — and the finished file is
// moved to finalPath on completion (see finalizeWorkingMove).
//
// The working path is deterministic given (TempDir, finalPath), so a resume in a
// later session computes the same path and finds the existing sidecar.
func resolveWorkingPath(opt Options, finalPath string) string {
	if opt.TempDir == "" || finalPath == "" {
		return finalPath
	}
	// MkdirAll is cheap + idempotent; do it here so the first WriteAt/OpenFile in
	// the engine does not fail on a missing temp dir.
	_ = os.MkdirAll(opt.TempDir, os.ModePerm)
	return filepath.Join(opt.TempDir, filepath.Base(finalPath))
}

// finalizeWorkingMove moves a completed download from its TempDir working path to
// the final output path when TempDir is in use, and clears any leftover sidecar
// at the working location. It is a no-op when TempDir is unset (working == final)
// or when the working path no longer exists (already moved). The job's output
// path is updated to finalPath so downstream steps (categorize, the API) act on
// the real file.
//
// It is called on the success path only — a failed/paused download keeps its
// partial under TempDir so it can be resumed/repaired.
func finalizeWorkingMove(opt Options, job *Job, workingPath, finalPath string) error {
	if opt.TempDir == "" || workingPath == finalPath {
		return nil
	}
	if _, err := os.Stat(workingPath); err != nil {
		if os.IsNotExist(err) {
			// Already moved (idempotent re-entry) or nothing to move.
			job.SetOutputPath(finalPath)
			return nil
		}
		return err
	}
	if err := os.MkdirAll(filepath.Dir(finalPath), os.ModePerm); err != nil {
		return fmt.Errorf("tempdir: create output dir: %w", err)
	}
	if err := moveFile(workingPath, finalPath); err != nil {
		return fmt.Errorf("tempdir: move finished file to output: %w", err)
	}
	// The resume sidecar (if any survived, e.g. when VerifyIntegrity kept it) is
	// tied to the working path; drop it so it doesn't linger in TempDir. A stored
	// full-file hash is preserved by re-pointing it next to the final file is a
	// follow-up; for now structural verification still works post-move.
	removePartsMeta(workingPath)
	job.SetOutputPath(finalPath)
	return nil
}

// RemovePartialData deletes the on-disk partial file at outputPath AND its
// .rumparts resume sidecar. It is used (a) on an explicit delete, which always
// removes partials, and (b) on a FAILED download when KeepPartialOnFailure is
// off. Missing files are ignored, so it is safe to call unconditionally. It must
// NOT be called for a successful or paused download (a pause keeps the partial
// so it can resume).
func RemovePartialData(outputPath string) {
	if outputPath == "" {
		return
	}
	removePartsMeta(outputPath)
	_ = os.Remove(outputPath)
}
