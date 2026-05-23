package filesystem

import (
	"log"
	"os"
	"os/exec"
	"path/filepath"

	"runtime"
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

func OpenFolder(fullFilePath string) error {
	dir := filepath.Dir(fullFilePath)
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "windows":
		cmd = exec.Command("explorer", dir)
	case "darwin":
		cmd = exec.Command("open", dir)
	default:
		cmd = exec.Command("xdg-open", dir)
	}

	return cmd.Run()
}
