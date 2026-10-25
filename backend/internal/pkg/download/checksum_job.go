package download

import (
	"context"
	"errors"
	"fmt"
	"os"
	"time"
)

// ChecksumUnavailable is the verdict when no checksum could be found to compare
// with. It is never stored on the job: "nothing to show" is the quiet state.
const ChecksumUnavailable = "unavailable"

// ChecksumOutcome is the result of an on-demand checksum check.
type ChecksumOutcome struct {
	Status string `json:"status"` // verified | mismatch | unavailable
	Algo   string `json:"algo,omitempty"`
	Source string `json:"source,omitempty"` // user | header | sidecar
}

// keepsFileOnFailure reports whether a failed download's file must be left
// alone whatever KeepPartialOnFailure says. A checksum mismatch means a whole
// file is on disk that the user may still want to inspect; only an explicit
// Redownload replaces it.
func keepsFileOnFailure(err error) bool { return errors.Is(err, ErrChecksumMismatch) }

// settleChecksum records how a finished (or failed) run compares with its
// checksum. With a user checksum the outcome follows from err. Without one, and
// unless disabled, it looks for a published checksum and checks the file against
// it. It never changes the job's status and never touches the file: a mismatch
// found here is only a flag the card shows.
func (m *JobManager) settleChecksum(ctx context.Context, job *Job, opt Options, err error) {
	if opt.Checksum != "" {
		switch {
		case err == nil:
			job.SetChecksumResult(ChecksumVerified, opt.ChecksumAlgo, ChecksumSourceUser)
		case errors.Is(err, ErrChecksumMismatch):
			job.SetChecksumResult(ChecksumMismatch, opt.ChecksumAlgo, ChecksumSourceUser)
		}
		return
	}
	if err != nil {
		return
	}
	job.SetChecksumResult("", "", "") // a fresh copy: drop any verdict on the old one
	if opt.NoAutoChecksum || ctx.Err() != nil || opt.Downloader == nil {
		return
	}
	res, derr := m.checkAgainstPublished(ctx, job, opt)
	if derr == nil && res.Status != ChecksumUnavailable {
		job.SetChecksumResult(res.Status, res.Algo, res.Source)
	}
}

// checkAgainstPublished discovers a published checksum for the job's URL and
// compares the file with it.
func (m *JobManager) checkAgainstPublished(ctx context.Context, job *Job, opt Options) (ChecksumOutcome, error) {
	path := job.GetOutputPath()
	if path == "" {
		return ChecksumOutcome{Status: ChecksumUnavailable}, nil
	}
	dg, ok := DiscoverChecksum(ctx, opt.Downloader, job.GetURL(), job.GetFileName())
	if !ok {
		return ChecksumOutcome{Status: ChecksumUnavailable}, nil
	}
	return compareFile(path, dg.Algo, dg.Hex, dg.Source)
}

func compareFile(path, algo, want, source string) (ChecksumOutcome, error) {
	out := ChecksumOutcome{Algo: algo, Source: source}
	err := VerifyChecksum(path, algo, want)
	switch {
	case err == nil:
		out.Status = ChecksumVerified
	case errors.Is(err, ErrChecksumMismatch):
		out.Status = ChecksumMismatch
	default:
		return ChecksumOutcome{}, err
	}
	return out, nil
}

// VerifyChecksumJob checks a finished file against its checksum on demand: the
// one supplied with the download, else one published beside it. A missing
// checksum is reported as ChecksumUnavailable and recorded nowhere. The file is
// only read.
func (m *JobManager) VerifyChecksumJob(ctx context.Context, id string) (ChecksumOutcome, error) {
	job, ok := m.GetJob(id)
	if !ok {
		return ChecksumOutcome{}, fmt.Errorf("job %s not found: %w", id, ErrNotFound)
	}
	if job.GetStatus() == StatusRunning {
		return ChecksumOutcome{}, fmt.Errorf("job %s is downloading: %w", id, ErrConflict)
	}
	path := job.GetOutputPath()
	if _, err := os.Stat(path); path == "" || err != nil {
		return ChecksumOutcome{}, fmt.Errorf("job %s: file is not on disk: %w", id, ErrNotFound)
	}

	var res ChecksumOutcome
	var err error
	if cs := job.GetChecksum(); cs != "" {
		res, err = compareFile(path, job.GetChecksumAlgo(), cs, ChecksumSourceUser)
	} else {
		res, err = m.checkAgainstPublished(ctx, job, m.effectiveOptsForJob(job))
	}
	if err != nil {
		return ChecksumOutcome{}, err
	}
	if res.Status != ChecksumUnavailable {
		job.SetChecksumResult(res.Status, res.Algo, res.Source)
		m.saveToDisk()
	}
	return res, nil
}

// RedownloadJob replaces a finished or damaged file: it deletes the file on
// disk, then queues the same job again with its URL and options unchanged. This
// is the only path that removes a file that failed a checksum, and it runs only
// on the user's request.
func (m *JobManager) RedownloadJob(ctx context.Context, id string) error {
	m.mu.Lock()
	job, ok := m.jobs[id]
	if !ok {
		m.mu.Unlock()
		return fmt.Errorf("job %s not found: %w", id, ErrNotFound)
	}
	if job.GetStatus() == StatusRunning {
		m.mu.Unlock()
		return fmt.Errorf("job %s is already running: %w", id, ErrConflict)
	}
	RemovePartialData(job.GetOutputPath())
	job.SetDownloaded(0)
	job.SetError(nil)
	job.SetChecksumResult("", "", "")
	job.SetIntegrityHash("", "")
	job.SetCompletedAt(time.Time{})
	job.SetStatus(StatusPending)
	m.mu.Unlock()

	return m.StartJob(ctx, id)
}
