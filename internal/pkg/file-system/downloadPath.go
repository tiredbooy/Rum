package filesystem

import (
	"log"
	"os"
	"path"
)

func getHomeDir() string {
	// For Linux/macOS
	if home := os.Getenv("HOME"); home != "" {
		return home
	}
	// For Windows
	if home := os.Getenv("USERPROFILE"); home != "" {
		return home
	}

	log.Fatal("Could not find user's home directory")
	return ""
}

func GetOrCreateDirectory() string {
	homeDir := getHomeDir()
	appName := "Rum"
	downloadDir := path.Join(homeDir, "Downloads", appName)

	_, err := os.Stat(downloadDir)
	if os.IsNotExist(err) {
		err := os.MkdirAll(downloadDir, 0755)
		if err != nil {
			log.Println("Failed to Create Directory")
			return ""
		}
	}

	return downloadDir
}
