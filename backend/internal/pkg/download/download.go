package download

import (
	"context"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"

	"github.com/tiredbooy/Rum/backend/internal/pkg/config"
	filesystem "github.com/tiredbooy/Rum/backend/internal/pkg/file-system"
	"github.com/tiredbooy/Rum/backend/internal/pkg/format"
	"github.com/tiredbooy/Rum/backend/internal/pkg/utils"
)

type ProgressFunc func(downloaded, total int64)

// progressPercent computes a download completion percentage clamped to [0,100].
// When total <= 0 the size is unknown ("indeterminate") and this returns -1 so
// callers can detect that case instead of producing NaN/garbage from a divide
// by zero or negative total. A non-negative total always yields a value in
// [0,100].
func progressPercent(downloaded, total int64) int {
	if total <= 0 {
		return -1 // indeterminate
	}
	pct := int(float64(downloaded) / float64(total) * 100)
	if pct < 0 {
		return 0
	}
	if pct > 100 {
		return 100
	}
	return pct
}

func PrepareOutputPath(opt Options, fileName, url string, contentType string) (fullPath string) {
	// Sanitize the remote-derived filename to prevent path traversal: a name
	// like "../../etc/passwd" (from a URL path or Content-Disposition) must not
	// escape the download directory.
	fileName = filesystem.SanitizeFileName(fileName)

	var setting config.Setting
	setting.LoadSettingMetadata()
	defaultDownloadDir := setting.OutDir

	var folderName string = ""
	if opt.Out != defaultDownloadDir {
		folderName = ""
	} else {
		folderName = format.GetFolderName(contentType)
	}

	fullFolderPath := filepath.Join(opt.Out, folderName)
	os.MkdirAll(fullFolderPath, os.ModePerm)

	groupFolderPath := filepath.Join(fullFolderPath, opt.GroupFolder)
	filesystem.CreateGroupFolder(groupFolderPath)

	if opt.WantGroupFolder {
		fullPath = filepath.Join(groupFolderPath, fileName)
	} else {
		fullPath = filepath.Join(fullFolderPath, fileName)
	}

	return fullPath
}

func DownloadWithRange(ctx context.Context, opt Options, req *http.Request, fileName string, outFile *os.File, offset int64, job *Job, progressFn ProgressFunc) error {
	if offset > 0 {
		req.Header.Set("Range", fmt.Sprintf("bytes=%d-", offset))
	}

	resp, err := opt.Downloader.Client.Do(req.WithContext(ctx))
	if err != nil {
		return err
	}

	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusPartialContent {
		if offset > 0 && resp.StatusCode == http.StatusRequestedRangeNotSatisfiable {
			job.SetDownloaded(offset)
			job.SetTotalSize(offset)
			job.SetStatus(StatusCompleted)
			return nil
		}
		return &httpStatusError{code: resp.StatusCode}
	}

	if offset > 0 && resp.StatusCode == http.StatusOK {
		if err := outFile.Truncate(0); err != nil {
			return err
		}
		if _, err := outFile.Seek(0, io.SeekStart); err != nil {
			return err
		}
		offset = 0
	}

	var body io.ReadCloser = resp.Body
	if opt.Governor != nil {
		// Shared, live-adjustable global cap (desktop server path). The governor
		// enforces one budget across all concurrent downloads and honors live
		// bandwidth-window changes.
		body = opt.Governor.Wrap(ctx, resp.Body)
	} else {
		// Engine/CLI path: per-download limiter. Per-job override (opt.SpeedLimit,
		// KB/s) takes precedence; otherwise fall back to the global setting.
		var setting config.Setting
		if err := setting.LoadSettingMetadata(); err != nil {
			log.Println("Failed to load setting metadata.")
		}
		limitKB := setting.SpeedLimitKB
		if opt.SpeedLimit > 0 {
			limitKB = opt.SpeedLimit
		}
		if limiter := newSpeedLimiter(limitKB); limiter != nil {
			body = &rateLimitedReader{
				reader:  resp.Body,
				limiter: limiter,
				ctx:     ctx,
			}
		}
	}

	resp.Body = body

	return SaveDownloadedFile(ctx, resp, outFile, offset, fileName, job, progressFn)
}

func SaveDownloadedFile(ctx context.Context, resp *http.Response, outFile *os.File, existsFileSize int64, fileName string, job *Job, progressFn ProgressFunc) error {
	if _, err := outFile.Seek(existsFileSize, io.SeekStart); err != nil {
		return err
	}

	remainingSize := resp.ContentLength
	var totalSize int64

	if remainingSize > 0 {
		totalSize = existsFileSize + remainingSize
	} else {
		totalSize = -1
	}

	buffer := make([]byte, downloadBufferSize)
	var downloaded int64 = existsFileSize

	for {
		n, err := resp.Body.Read(buffer)
		if n > 0 {
			if _, werr := outFile.Write(buffer[:n]); werr != nil {
				return werr
			}
			downloaded += int64(n)
			job.SetDownloaded(downloaded)
			if progressFn != nil {
				progressFn(downloaded, totalSize)
			}
		}
		if err == io.EOF {
			if syncErr := outFile.Sync(); syncErr != nil {
				return syncErr
			}
			return nil
		}
		if err != nil {
			if ctx.Err() != nil {
				return ctx.Err()
			}
			return err
		}
	}
}

func DownloadSingleFile(ctx context.Context, opt Options, job *Job, progressFn ProgressFunc) error {
	url := utils.UrlValidation(job.URL)
	fullPath := PrepareOutputPath(opt, job.FileName, url, job.ContentType)
	job.SetOutputPath(fullPath)

	var existsFileSize int64 = 0
	if filesystem.IsFileExists(fullPath) {
		gotExistsFileSize, err := filesystem.GetExistsFileSize(fullPath)
		if err != nil {
			return err
		}
		existsFileSize = gotExistsFileSize
	}

	if job.TotalSize <= 0 && existsFileSize > 0 {
		DebugLog("Remote size unknown, local file exists -> marking as completed")
		// Even a pre-existing file is verified against a supplied checksum (no-op
		// when none was supplied).
		if err := VerifyChecksum(fullPath, opt.ChecksumAlgo, opt.Checksum); err != nil {
			return err
		}
		job.SetStatus(StatusCompleted)
		job.SetDownloaded(existsFileSize)
		return nil
	}

	if job.TotalSize >= 1 && existsFileSize == int64(job.TotalSize) {
		DebugLog("Found Completed File Pass")
		if err := VerifyChecksum(fullPath, opt.ChecksumAlgo, opt.Checksum); err != nil {
			return err
		}
		job.SetStatus(StatusCompleted)
		job.SetDownloaded(int64(job.TotalSize))
		return nil
	}

	if job.TotalSize > 0 {
		job.SetTotalSize(int64(job.TotalSize))
	} else {
		job.SetTotalSize(-1)
	}

	// Fail fast if the destination filesystem clearly cannot hold what remains to
	// be downloaded, instead of allocating, downloading partway, and dying with
	// ENOSPC. Only the not-yet-downloaded bytes need to fit. Degrades to a no-op
	// when the size is unknown or free space can't be determined.
	if job.TotalSize > 0 {
		remaining := int64(job.TotalSize) - existsFileSize
		if remaining > 0 {
			if err := PreflightDiskSpace(filepath.Dir(fullPath), remaining); err != nil {
				return err
			}
		}
	}

	// Wrap the actual transfer in retry-with-backoff. Each attempt re-stats the
	// partial file and resumes from its current size, so a transient failure
	// mid-stream resumes where it left off instead of restarting. Permanent
	// errors (HTTP 4xx) and context cancellation are not retried.
	cfg := newRetryConfig(opt.MaxRetries)
	if err := retryWithBackoff(ctx, cfg, func(ctx context.Context) error {
		return downloadSingleAttempt(ctx, opt, job, url, fullPath, progressFn)
	}); err != nil {
		return err
	}

	// Verify integrity once, after a successful assemble (NOT per retry attempt).
	// A mismatch surfaces ErrChecksumMismatch and leaves the file in place so the
	// user can inspect/retry. No-op when opt.Checksum == "".
	if err := VerifyChecksum(fullPath, opt.ChecksumAlgo, opt.Checksum); err != nil {
		return err
	}

	// Auto-organize into a category directory (no-op unless opt.Categorize and a
	// matching rule). A categorize failure must not lose the downloaded file, so
	// log and continue rather than returning an error.
	if err := finalizeCategorize(opt, job); err != nil {
		DebugLog("categorize failed: " + err.Error())
	}

	return nil
}

// downloadSingleAttempt performs one single-stream download attempt, resuming
// from whatever bytes are already on disk.
func downloadSingleAttempt(ctx context.Context, opt Options, job *Job, url, fullPath string, progressFn ProgressFunc) error {
	var existsFileSize int64 = 0
	if filesystem.IsFileExists(fullPath) {
		if sz, err := filesystem.GetExistsFileSize(fullPath); err == nil {
			existsFileSize = sz
		}
	}

	outFile, err := os.OpenFile(fullPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		return err
	}
	defer outFile.Close()

	req, err := opt.Downloader.NewRequest("GET", url)
	if err != nil {
		return err
	}
	req = req.WithContext(ctx)

	if existsFileSize > 0 && !job.SupportRange {
		fmt.Println("Server does not support range. Starting over...")
		if err := outFile.Truncate(0); err != nil {
			return err
		}
		if _, err := outFile.Seek(0, io.SeekStart); err != nil {
			return err
		}
		existsFileSize = 0
	}

	if existsFileSize > 0 {
		DebugLog("Trying to Resume Exists File")
	}

	return DownloadWithRange(ctx, opt, req, job.FileName, outFile, existsFileSize, job, progressFn)
}
