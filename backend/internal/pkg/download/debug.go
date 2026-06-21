package download

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"time"
)

var debugFile *os.File

func InitLogFile() error {
	path, err := os.UserConfigDir()
	if err != nil {
		log.Println("Failed to get user config path")
		return err
	}

	dir := filepath.Join(path, "rum", "logs")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	filePath := filepath.Join(dir, "debug.log")

	// The error check was inverted (`if err != nil { debugFile = f }`), so the
	// handle was only ever stored on failure — leaving debugFile nil and DebugLog
	// a permanent no-op. Append (not truncate) so debug history survives restarts.
	f, err := os.OpenFile(filePath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		return err
	}
	debugFile = f

	return nil
}

func DebugLog(msg string) {
	if debugFile != nil {
		debugFile.WriteString(fmt.Sprintf("%s: %s\n", time.Now().Format("15:04:05"), msg))
		debugFile.Sync()
	}
}
