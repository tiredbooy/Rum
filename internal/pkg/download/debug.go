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

	filePath := filepath.Join(path, "rum", "debug.log")

	f, err := os.OpenFile(filePath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0644)
	if err != nil {
		debugFile = f
	}

	log.Printf("Log Initilized At %s", filePath)

	return nil
}

func DebugLog(msg string) {
	if debugFile != nil {
		debugFile.WriteString(fmt.Sprintf("%s: %s\n", time.Now().Format("15:04:05"), msg))
		debugFile.Sync()
	}
}
