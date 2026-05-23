package download

import (
	"context"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"

	"golang.org/x/time/rate"

	"github.com/tiredbooy/Rum/backend/internal/pkg/config"
	filesystem "github.com/tiredbooy/Rum/backend/internal/pkg/file-system"
	"github.com/tiredbooy/Rum/backend/internal/pkg/format"
	"github.com/tiredbooy/Rum/backend/internal/pkg/utils"
)

type ProgressFunc func(downloaded, total int64)

func PrepareOutputPath(opt Options, fileName, url string, contentType string) (fullPath string) {
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
		return fmt.Errorf("unexpected status: %d", resp.StatusCode)
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

	var setting config.Setting
	err = setting.LoadSettingMetadata()
	if err != nil {
		log.Println("Failed to load setting metadata.")
	}

	var body io.ReadCloser = resp.Body
	if setting.SpeedLimitKB > 0 {
		var speedLimit int64
		if opt.SpeedLimit*1024 > 0 && opt.SpeedLimit == setting.SpeedLimitKB {
			speedLimit = int64(opt.SpeedLimit)
		}
		speedLimit = int64(setting.SpeedLimitKB) * 102
		limiter := rate.NewLimiter(rate.Limit(speedLimit), int(speedLimit))
		body = &rateLimitedReader{
			reader:  resp.Body,
			limiter: limiter,
			ctx:     ctx,
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
	job.OutputPath = fullPath

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
		job.SetStatus(StatusCompleted)
		job.SetDownloaded(existsFileSize)
		return nil
	}

	if job.TotalSize >= 1 && existsFileSize == int64(job.TotalSize) {
		DebugLog("Found Completed File Pass")
		job.SetStatus(StatusCompleted)
		job.SetDownloaded(int64(job.TotalSize))
		return nil
	}

	if job.TotalSize > 0 {
		job.SetTotalSize(int64(job.TotalSize))
	} else {
		job.SetTotalSize(-1)
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

	if existsFileSize > 0 {
		DebugLog("Trying to Resume Exists File")
		return DownloadWithRange(ctx, opt, req, job.FileName, outFile, existsFileSize, job, progressFn)
	}

	if existsFileSize > 0 && !job.SupportRange {
		fmt.Println("Server does not support range. Starting over...")
		if err := outFile.Truncate(0); err != nil {
			return err
		}
	}

	return DownloadWithRange(ctx, opt, req, job.FileName, outFile, 0, job, progressFn)
}
