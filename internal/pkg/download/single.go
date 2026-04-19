package download

import (
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	filesystem "swiftget.com/internal/pkg/file-system"
	"swiftget.com/internal/pkg/format"
)

func DownloadSingleFile(opt Options, rawUrl string) error {
	url := rawUrl
	if !strings.HasPrefix(rawUrl, "http://") && !strings.HasPrefix(rawUrl, "https://") {
		url = "https://" + rawUrl
	}

	headResp, err := GetHeader(url)
	if err != nil {
		log.Println(err.Error())
		return err
	}

	defer headResp.Body.Close()

	req, err := GetFile(url)
	if err != nil {
		log.Println("Failed to download: ", err.Error())
		return err
	}

	contentType := headResp.Header.Get("Content-Type")
	folderName := format.GetFolderName(contentType)

	fullFolderPath := filepath.Join(opt.Out, folderName)
	os.MkdirAll(fullFolderPath, os.ModePerm)

	groupFolderPath := filepath.Join(fullFolderPath, opt.GroupFolder)
	filesystem.CreateGroupFolder(groupFolderPath)

	fileName := format.CleanFileName(url)
	if fileName == "" || fileName == "/" {
		fileName = "downloaded.file"
	}

	var fullPath string
	if opt.WantGroupFolder {
		fullPath = filepath.Join(groupFolderPath, fileName)
	} else {
		fullPath = filepath.Join(fullFolderPath, fileName)
	}

	var existsFileSize int64 = 0
	if filesystem.IsFileExists(fullPath) {
		existsFileSize, err = filesystem.GetExistsFileSize(fullPath)
		if err != nil {
			log.Println("Failed to get size file", err.Error())
		}

		req.Header.Set("Range", fmt.Sprintf("bytes=%d-", existsFileSize))
		fmt.Printf("Resuming download at %s...\n", format.FormatSize(existsFileSize))
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}

	defer resp.Body.Close()

	if resp.StatusCode == http.StatusOK && existsFileSize > 0 {
		fmt.Println("Server did NOT support partial content. Restarting download...")
		os.Remove(fullPath)
		existsFileSize = 0
	}

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusPartialContent {
		return fmt.Errorf("unexpected status: %d", resp.StatusCode)
	}
	// New Method For Creating OR Appending File
	outFile, err := os.OpenFile(fullPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		return err
	}

	defer outFile.Close()

	fmt.Printf("\r=============================\n")
	remaining := resp.ContentLength
	totalSize := existsFileSize + remaining

	fmt.Printf("Downloading %s: total size %s\n",
		fileName, format.FormatSize(totalSize))

	if totalSize <= 0 {
		fmt.Println("Cannot get content length, progress bar disabled")
	}

	buffer := make([]byte, 64*1024)

	downloaded := existsFileSize
	start := time.Now()

	for {
		n, err := resp.Body.Read(buffer)
		if n > 0 {
			outFile.Write(buffer[:n])
			downloaded += int64(n)

			if totalSize > 0 {
				percent := float64(downloaded) / float64(totalSize) * 100
				elapsed := time.Since(start).Seconds()
				speed := float64(downloaded) / 1024 / elapsed

				fmt.Printf("\rDownloading... %.2f%% (%s of %s) at %s",
					percent, format.FormatSize(downloaded), format.FormatSize(totalSize), format.FormatSize(int64(speed*1024)))
			}
		}

		if err != nil {
			if err == io.EOF {
				break
			} else {
				return err
			}
		}
	}

	fmt.Printf("\r=============================\n")

	err = outFile.Sync()
	if err != nil {
		log.Println("Failed to Sync output File: ", err.Error())
		return err
	}

	return nil

}

// _, err = io.Copy(outFile, resp.Body)
// if err != nil {
// 	return nil
// }
