package filesystem

import (
	"log"
	"os"
)

func GetExistsFileSize(path string) (int64, error) {
	file, err := os.Stat(path)
	if err != nil {
		log.Println("Failed to get the file")
		return 0, err
	}

	fileSize := file.Size()

	return fileSize, nil
}

func IsFileExists(path string) bool {
	if path == "" {
		return false
	}

	_, err := os.Stat(path)
	if os.IsNotExist(err) {
		return false
	}

	return true
}
