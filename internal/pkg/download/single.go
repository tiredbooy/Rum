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

	"golang.org/x/time/rate"
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

func DownloadWithRange(ctx context.Context, downloader *Downloader, req *http.Request, fileName string, outFile *os.File, offset int64, job *Job, progressFn ProgressFunc) error {
	if offset > 0 {
		req.Header.Set("Range", fmt.Sprintf("bytes=%d-", offset))
	}

	resp, err := downloader.Client.Do(req.WithContext(ctx))
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

	var body io.ReadCloser = resp.Body
	if Opt.SpeedLimit > 0 {
		limiter := rate.NewLimiter(rate.Limit(Opt.SpeedLimit), Opt.SpeedLimit)
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

	// fmt.Printf("DEBUG SPEED: Opt.SpeedLimit = %d bytes/s, bufferSize will be %d\n",
	// 	Opt.SpeedLimit,
	// 	func() int64 {
	// 		if Opt.SpeedLimit > 0 {
	// 			bs := int64(Opt.SpeedLimit)
	// 			if bs < 32*1024 {
	// 				return 32 * 1024
	// 			}
	// 			if bs > 4*1024*1024 {
	// 				return 4 * 1024 * 1024
	// 			}
	// 			return bs
	// 		}
	// 		return 1024 * 1024
	// 	}())

	// if Opt.SpeedLimit > 0 && Opt.SpeedLimit < 10240 {
	// 	Opt.SpeedLimit = 10240
	// 	fmt.Printf("Speed limit too low, adjusted to minimum 10 KB/s (10240 bytes/s)\n")
	// }

	// bufferSize := int64(1024 * 1024)
	// if Opt.SpeedLimit > 0 {
	// 	bufferSize = int64(Opt.SpeedLimit)
	// 	if bufferSize < 32*1024 {
	// 		bufferSize = 32 * 1024
	// 	}
	// 	if bufferSize > 4*1024*1024 {
	// 		bufferSize = 4 * 1024 * 1024
	// 	}
	// }

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
			if ctx.Err() != nil {
				return ctx.Err()
			}
			return err
		}
	}
}

func DownloadSingleFile(ctx context.Context, opt Options, job *Job, progressFn ProgressFunc) error {
	url := utils.UrlValidation(job.URL)

	var referer, userAgent string

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

	downloader := NewDownloader(userAgent, referer)

	//fileInfo, err := GetHeaderInfo(url)
	fileInfo, err := downloader.HeadWithFallback(url)
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

	req, err := downloader.NewRequest("GET", url)

	if err != nil {
		return err
	}

	req = req.WithContext(ctx)

	if fileSize >= 1 && existsFileSize == fileSize {
		DebugLog("Found Completed File Pass")

		job.SetStatus(StatusCompleted)
		job.SetDownloaded(fileSize)
		return nil
	}

	if existsFileSize > 0 {
		DebugLog("Trying to Resume Exists File")
		return DownloadWithRange(ctx, downloader, req, fileName, outFile, existsFileSize, job, progressFn)
	}

	if existsFileSize > 0 && !fileInfo.SupportsRange {
		fmt.Println("Server does not support range. Starting over...")
		if err := outFile.Truncate(0); err != nil {
			return err
		}
	}

	return DownloadWithRange(ctx, downloader, req, fileName, outFile, 0, job, progressFn)
}
