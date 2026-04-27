package download

import (
	"context"
	"fmt"
	"io"
	"net/http"
	urlPkg "net/url"
	"os"
	"path/filepath"
	"strconv"

	filesystem "swiftget.com/internal/pkg/file-system"
	"swiftget.com/internal/pkg/format"
	"swiftget.com/internal/pkg/utils"
)

func PrepareOutputPath(opt Options, url string, contentType string) (fullPath, fileName string) {
	folderName := format.GetFolderName(contentType)

	fullFolderPath := filepath.Join(opt.Out, folderName)
	os.MkdirAll(fullFolderPath, os.ModePerm)

	groupFolderPath := filepath.Join(fullFolderPath, opt.GroupFolder)
	filesystem.CreateGroupFolder(groupFolderPath)

	fileName = format.ExtractFileNameFromURL(url)
	if fileName == "" {
		fileName = format.CleanFileName(url)
	}

	if fileName == "" || fileName == "/" {
		fileName = "downloaded.file"
	}

	if opt.WantGroupFolder {
		fullPath = filepath.Join(groupFolderPath, fileName)
	} else {
		fullPath = filepath.Join(fullFolderPath, fileName)
	}

	return fullPath, fileName
}

func DownloadWithRange(ctx context.Context, req *http.Request, fileName string, outFile *os.File, offset int64, job *Job, progressFn ProgressFunc) error {
	if offset > 0 {
		req.Header.Set("Range", fmt.Sprintf("bytes=%d-", offset))
	}

	req = req.WithContext(ctx)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusPartialContent {
		return fmt.Errorf("unexpected status: %d", resp.StatusCode)
	}

	if offset > 0 && resp.StatusCode == http.StatusOK {
		fmt.Println("Server does not support partial content. Restarting download...")
		if err := outFile.Truncate(0); err != nil {
			return err
		}
		if _, err := outFile.Seek(0, io.SeekStart); err != nil {
			return err
		}
		offset = 0
	}

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

	buffer := make([]byte, 1024*1024) // 1MB
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
			if downloaded%500 == 0 {
				go SaveJobsToDisk()
			}
		}
		if err == io.EOF {
			if syncErr := outFile.Sync(); syncErr != nil {
				return syncErr
			}
			return nil
		}
		if err != nil {
			// If the context was cancelled, return that error
			if ctx.Err() != nil {
				return ctx.Err()
			}
			return err
		}
	}
}

func DownloadSingleFile(ctx context.Context, opt Options, job *Job, progressFn ProgressFunc) error {
	url := utils.UrlValidation(job.URL)

	var referer string
	var userAgent string

	if opt.Referer != "" {
		referer = opt.Referer
	} else {
		u, _ := urlPkg.Parse(job.URL)
		referer = fmt.Sprintf("%s://%s/", u.Scheme, u.Host)
	}

	if opt.UserAgent != "" {
		userAgent = opt.UserAgent
	} else {
		userAgent = utils.GetRandomUserAgent()
	}

	fileInfo, err := GetHeaderInfo(url)
	if err != nil {
		return err
	}

	fileSize, _ := strconv.ParseInt(fileInfo.ContentSize, 64, 10)

	if fileSize > 0 {
		job.SetTotalSize(fileSize)
	} else {
		job.SetTotalSize(-1)
	}

	fullPath, fileName := PrepareOutputPath(opt, url, fileInfo.ContentType)
	job.SetFileName(fileName)

	outFile, err := os.OpenFile(fullPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		return err
	}

	defer outFile.Close()

	var existsFileSize int64 = 0
	if filesystem.IsFileExists(fullPath) {
		existsFileSize, err = filesystem.GetExistsFileSize(fullPath)
		if err != nil {
			return err
		}
	}

	req, err := GetFile(url)

	if err != nil {
		return err
	}

	req = req.WithContext(ctx)

	req.Header.Set("User-Agent", userAgent)
	req.Header.Set("Referer", referer)
	req.Header.Set("Accept-Language", "en-US,en;q=0.5")
	req.Header.Set("Accept-Encoding", "gzip, deflate, br")
	req.Header.Set("Connection", "keep-alive")

	if existsFileSize == fileSize {
		DebugLog("Found Completed File Pass")

		job.SetStatus(StatusCompleted)
		job.SetDownloaded(fileSize)
		return nil
	}

	if existsFileSize > 0 && fileInfo.SupportsRange {
		DebugLog("Trying to Resume Exists File")
		return DownloadWithRange(ctx, req, fileName, outFile, existsFileSize, job, progressFn)
	}

	if existsFileSize > 0 && !fileInfo.SupportsRange {
		fmt.Println("Server does not support range. Starting over...")
		if err := outFile.Truncate(0); err != nil {
			return err
		}
	}

	return DownloadWithRange(ctx, req, fileName, outFile, 0, job, progressFn)
}
