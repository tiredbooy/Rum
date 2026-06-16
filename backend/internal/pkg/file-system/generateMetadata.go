package filesystem

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
)

// func CreateFile(fileName string, )

func CreateMetadataFile(fileName string) string {
	configDir, err := os.UserConfigDir()
	if err != nil {
		configDir = ".rum"
	}
	dir := filepath.Join(configDir, "rum")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		log.Printf("filesystem: failed to create metadata dir %s: %v", dir, err)
	}
	return filepath.Join(dir, fileName)
}

func ReadMetadataFile(fileName string) ([]byte, error) {
	path := CreateMetadataFile(fileName)
	return os.ReadFile(path)
}

func WriteMetadataFile(filename string, data interface{}) error {
	path := CreateMetadataFile(filename)
	fileData, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal %s: %w", filename, err)
	}
	// Atomic write: a crash mid-write can no longer corrupt jobs.json/settings.json.
	if err := AtomicWriteFile(path, fileData, 0o644); err != nil {
		return fmt.Errorf("write %s: %w", filename, err)
	}
	return nil
}
