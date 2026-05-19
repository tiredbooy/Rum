package filesystem

import (
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
