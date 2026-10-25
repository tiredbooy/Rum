package download

import (
	"errors"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/tiredbooy/Rum/backend/internal/pkg/format"
)

// diskFreeFn reports the free bytes available at dir. A variable so tests can
// play the part of a volume that fills up and later frees space.
var diskFreeFn = availableDiskBytes

// testWriteFault, when non-nil, is consulted with the size of every data write
// before it reaches the file; a non-nil result is returned as the write error.
// It exists only so tests can inject ENOSPC mid-download; production leaves it
// nil.
var testWriteFault func(n int) error

// DiskFullError says a download cannot continue because the destination volume
// is out of space. It is the single error the engine uses for that condition,
// whether found by the preflight check or by a failed write/sync/rename.
type DiskFullError struct {
	// NeededBytes is how many more bytes the download still has to write.
	NeededBytes int64
	// FreeBytes is the free space seen on the volume when the error was raised.
	FreeBytes int64
	// Path is the directory on the affected volume.
	Path string
	// Err is the underlying cause (ENOSPC, EDQUOT, ...), if any.
	Err error
}

func (e *DiskFullError) Error() string {
	if e == nil {
		return "<nil>"
	}
	msg := "not enough disk space"
	if e.NeededBytes > 0 {
		msg += fmt.Sprintf(": need %s more", format.FormatBytes(e.NeededBytes))
		msg += fmt.Sprintf(", %s free", format.FormatBytes(e.FreeBytes))
	}
	if e.Path != "" {
		msg += " in " + e.Path
	}
	return msg
}

// Unwrap lets errors.Is match both the taxonomy sentinel and the OS error.
func (e *DiskFullError) Unwrap() []error {
	if e == nil {
		return nil
	}
	if e.Err == nil {
		return []error{ErrInsufficientDiskSpace}
	}
	return []error{ErrInsufficientDiskSpace, e.Err}
}

// ErrorCodeOf returns the machine code for err, or "" when it has none.
func ErrorCodeOf(err error) string {
	var df *DiskFullError
	if errors.As(err, &df) {
		return ErrorCodeDiskFull
	}
	return ""
}

// AsDiskFull returns the *DiskFullError inside err, if any.
func AsDiskFull(err error) (*DiskFullError, bool) {
	var df *DiskFullError
	if errors.As(err, &df) && df != nil {
		return df, true
	}
	return nil, false
}

// isNoSpaceError reports whether err means "the volume is full": ENOSPC or a
// quota error on unix, ERROR_DISK_FULL / ERROR_HANDLE_DISK_FULL on Windows
// (see diskfull_*.go). The text match is a last resort for wrapped errors that
// lost their errno.
func isNoSpaceError(err error) bool {
	if err == nil {
		return false
	}
	if isNoSpaceErrno(err) {
		return true
	}
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "no space left on device") ||
		strings.Contains(msg, "disk quota exceeded") ||
		strings.Contains(msg, "not enough space on the disk")
}

// asDiskFull upgrades a raw out-of-space error into a *DiskFullError carrying
// the bytes still needed and the volume's current free space. Any other error
// (and an existing *DiskFullError) is returned unchanged.
func asDiskFull(err error, dir string, neededBytes int64) error {
	if err == nil {
		return nil
	}
	if _, ok := AsDiskFull(err); ok || !isNoSpaceError(err) {
		return err
	}
	free, _ := diskFreeFn(dir)
	return &DiskFullError{NeededBytes: neededBytes, FreeBytes: free, Path: dir, Err: err}
}

// classifyDiskFull turns a raw out-of-space error that ended a running job into
// a *DiskFullError sized from the job's progress: what is still to download, or
// the whole file when the failure came after the last byte (the final move).
func classifyDiskFull(err error, job *Job) error {
	if _, ok := AsDiskFull(err); ok || !isNoSpaceError(err) {
		return err
	}
	total, done := job.GetTotalSize(), job.GetDownloaded()
	needed := total - done
	if total > 0 && needed <= 0 {
		needed = total
	}
	if needed < 0 {
		needed = 0
	}
	return asDiskFull(err, filepath.Dir(job.GetOutputPath()), needed)
}
