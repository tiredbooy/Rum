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
	// Last resort: Go's portable lookup. Never fatal — killing the process here
	// would take down the engine/api/tui that depend on this package.
	if home, err := os.UserHomeDir(); err == nil && home != "" {
		return home
	}
	log.Println("filesystem: could not determine home directory; using current directory")
	return "."
}

func GetOrCreateDownloadDirectory() string {
	homeDir := getHomeDir()
	appName := "Rum"
	downloadDir := path.Join(homeDir, "Downloads", appName)

	// MkdirAll is idempotent; call unconditionally so a stat error doesn't hide a
	// missing dir.
	if err := os.MkdirAll(downloadDir, 0o755); err != nil {
		log.Printf("filesystem: failed to create download directory %s: %v", downloadDir, err)
		return ""
	}

	return downloadDir
}
