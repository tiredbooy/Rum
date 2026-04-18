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
	var url string

	if strings.HasPrefix(rawUrl, "https://") || strings.HasPrefix(rawUrl, "http://") {
		url = rawUrl
	} else {
		url = fmt.Sprintf("%s%s", "https://", rawUrl)
	}

	log.Println("URL: ", url)
	resp, err := http.Get(url)
	if err != nil {
		log.Println("Failed to Download the file: ", err.Error())
		return err
	}

	defer resp.Body.Close()

	fileContentType := resp.Header.Get("Content-Type")

	folderName := format.GetFolderName(fileContentType)
	fullFolderPath := filepath.Join(opt.Out, folderName)
	os.MkdirAll(fullFolderPath, os.ModePerm)

	groupFolderName := filepath.Join(fullFolderPath, opt.GroupFolder)
	filesystem.CreateGroupFolder(groupFolderName)

	fileName := format.CleanFileName(url)

	if fileName == "" || fileName == "/" {
		fileName = "downloaded.file"
	}

	fmt.Printf("\r %s Started Downloading \n", fileName)

	var fullPath string
	if opt.WantGroupFolder {
		fullPath = filepath.Join(groupFolderName, fileName)
	} else {
		fullPath = filepath.Join(fullFolderPath, fileName)
	}

	outFile, err := os.Create(fullPath)
	if err != nil {
		log.Println("Error Creating file: ", err)
		return err
	}

	defer outFile.Close()

	fileSize := resp.ContentLength
	fmt.Printf("\r=============================\n")
	fmt.Println("FILE URL: ", url)
	fmt.Printf("\rFileName: %s, FileSize: %s\n", fileName, format.FormatSize(fileSize))

	if fileSize <= 0 {
		fmt.Println("Cannot get content length, progress bar disabled")
	}

	buffer := make([]byte, 64*1024)
	var downloaded int64 = 0

	start := time.Now()

	for {
		n, err := resp.Body.Read(buffer)
		if n > 0 {
			outFile.Write(buffer[:n])
			downloaded += int64(n)

			if fileSize > 0 {
				percent := float64(downloaded) / float64(fileSize) * 100
				elapsed := time.Since(start).Seconds()
				speed := float64(downloaded) / 1024 / elapsed

				fmt.Printf("\rDownloading... %.2f%% (%s of %s) at %s",
					percent, format.FormatSize(downloaded), format.FormatSize(fileSize), format.FormatSize(int64(speed*1024)))
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

	_, err = io.Copy(outFile, resp.Body)
	if err != nil {
		return nil
	}

	return nil

}
