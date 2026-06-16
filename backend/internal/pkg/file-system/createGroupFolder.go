package filesystem

import (
	"log"
	"os"
)

func CreateGroupFolder(folderPath string) {
	if folderPath == "" {
		return
	}

	// MkdirAll is a no-op if the directory already exists, so we can call it
	// unconditionally and surface any real error (permissions, file-in-the-way).
	if err := os.MkdirAll(folderPath, 0o755); err != nil {
		log.Printf("filesystem: failed to create directory %s: %v", folderPath, err)
	}
}
