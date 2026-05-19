package filesystem

import (
	"encoding/json"
	"fmt"
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
	os.MkdirAll(dir, 0755)
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
	if err := os.WriteFile(path, fileData, 0644); err != nil {
		return fmt.Errorf("write %s: %w", filename, err)
	}
	return nil
}
