package download

import (
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"swiftget.com/internal/pkg/format"
)

func StartDownload(opt Options) error {
	resp, err := http.Get(opt.URL)
	if err != nil {
		return err
	}

	defer resp.Body.Close()

	fileContentType := resp.Header.Get("Content-Type")

	folderName := format.GetFolderName(fileContentType)
	fullFolderPath := filepath.Join(opt.Out, folderName)
	os.MkdirAll(fullFolderPath, os.ModePerm)

	fileName := format.CleanFileName(opt.URL)

	if fileName == "" || fileName == "/" {
		fileName = "downloaded.file"
	}

	fullPath := filepath.Join(fullFolderPath, fileName)

	outFile, err := os.Create(fullPath)
	if err != nil {
		log.Println("Error Creating file: ", err)
		return err
	}

	defer outFile.Close()

	fileSize := resp.ContentLength
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
