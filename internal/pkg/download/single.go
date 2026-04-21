package download

import (
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	filesystem "swiftget.com/internal/pkg/file-system"
	"swiftget.com/internal/pkg/format"
	"swiftget.com/internal/pkg/utils"
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

	//fmt.Printf("\r=============================\n")
	fmt.Println("=== Downloading ===")
	remaining := resp.ContentLength
	totalSize := existsFileSize + remaining

	bar := utils.NewProgressBar(totalSize, fileName)

	buffer := make([]byte, 64*1024)

	for {
		n, err := resp.Body.Read(buffer)
		if n > 0 {
			outFile.Write(buffer[:n])
			utils.UpdateProgress(bar, int64(n)) // Add64 که مقدار اضافه می‌کنه
		}
		if err != nil {
			if err == io.EOF {
				break
			}
			return err
		}
	}

	utils.FinishProgress(bar)
	//fmt.Printf("\r=============================\n")

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
