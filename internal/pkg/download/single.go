package download

import (
	"context"
	"fmt"
	"io"
	"net/http"
	urlPkg "net/url"
	"os"
	"path/filepath"

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

func DownloadWithRange(ctx context.Context, req *http.Request, fileName string, outFile *os.File, offset int64) error {
	if offset > 0 {
		req.Header.Set("Range", fmt.Sprintf("bytes=%d-", offset))
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}

	if resp != nil {
		defer resp.Body.Close()
	} else {
		return fmt.Errorf("Http Client returned a nil response")
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

	return SaveDownloadedFile(ctx, resp, outFile, offset, fileName)
}

func SaveDownloadedFile(ctx context.Context, resp *http.Response, outFile *os.File, existsFileSize int64, fileName string) error {

	if _, err := outFile.Seek(existsFileSize, io.SeekStart); err != nil {
		return err
	}

	remainingSize := resp.ContentLength
	totalSize := existsFileSize + remainingSize

	bar := utils.NewProgressBar(totalSize, existsFileSize, fileName)

	buffer := make([]byte, 64*1024)

	for {
		select {
		case <-ctx.Done():
			return nil
		default:
			n, err := resp.Body.Read(buffer)
			if n > 0 {
				if _, err := outFile.Write(buffer[:n]); err != nil {
					return err
				}
				utils.UpdateProgress(bar, int64(n))
			}

			if err == io.EOF {
				utils.FinishProgress(bar, fileName)
				if syncErr := outFile.Sync(); syncErr != nil {
					return syncErr
				}
				return nil
			}
			if err != nil {
				utils.FinishProgress(bar, fileName)

				return err
			}
		}

	}

}

func DownloadSingleFile(ctx context.Context, opt Options, rawUrl string) error {
	url := utils.UrlValidation(rawUrl)
	var referer string
	if opt.Referer != "" {
		referer = opt.Referer
	} else {
		u, _ := urlPkg.Parse(rawUrl)
		referer = fmt.Sprintf("%s://%s/", u.Scheme, u.Host)
	}

	fileInfo, err := GetHeaderInfo(url)
	if err != nil {
		return err
	}

	fullPath, fileName := PrepareOutputPath(opt, url, fileInfo.ContentType)

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

	req.Header.Set("User-Agent", utils.GetRandomUserAgent())
	req.Header.Set("Referer", referer)
	req.Header.Set("Accept-Language", "en-US,en;q=0.5")
	req.Header.Set("Accept-Encoding", "gzip, deflate, br")
	req.Header.Set("Connection", "keep-alive")

	// fmt.Println("DEBUG: about to use variable Resp")
	// if req != nil {
	// 	defer req.Body.Close()
	// } else {
	// 	fmt.Println("Failed to get Request")
	// 	return err
	// }

	if existsFileSize > 0 && fileInfo.SupportsRange {
		fmt.Printf("Resuming download at %s...\n", format.FormatSize(existsFileSize))
		return DownloadWithRange(ctx, req, fileName, outFile, existsFileSize)
	}

	if existsFileSize > 0 && !fileInfo.SupportsRange {
		fmt.Println("Server does not support range. Starting over...")
		if err := outFile.Truncate(0); err != nil {
			return err
		}
	}

	return DownloadWithRange(ctx, req, fileName, outFile, 0)
}

// fmt.Println("=== Downloading ===")
// remaining := resp.ContentLength
// totalSize := existsFileSize + remaining

// bar := utils.NewProgressBar(totalSize, existsFileSize, fileName)

// buffer := make([]byte, 64*1024)

// for {
// 	select {
// 	case <-ctx.Done():
// 		return nil

// 	default:
// 		n, err := resp.Body.Read(buffer)
// 		if n > 0 {
// 			outFile.Write(buffer[:n])
// 			utils.UpdateProgress(bar, int64(n))
// 			// downloaded += int64(n)
// 		}
// 		if err != nil {
// 			if err == io.EOF {
// 				break
// 			}
// 			return err
// 		}
// 	}

// 	utils.FinishProgress(bar, fileName)

// 	err = outFile.Sync()
// 	if err != nil {
// 		log.Println("Failed to Sync output File: ", err.Error())
// 		return err
// 	}

// 	return nil

// }
