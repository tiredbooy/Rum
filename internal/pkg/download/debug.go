package download

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"time"
)

var debugFile *os.File

func InitLogFile() {
	path, err := os.UserConfigDir()
	if err != nil {
		log.Println("Failed to get user config path")
		return
	}

	filePath := filepath.Join(path, "debug.log")

	f, err := os.OpenFile(filePath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0644)
	if err != nil {
		debugFile = f
	}

	log.Printf("Log Initilized At %s", filePath)
}

func DebugLog(msg string) {
	if debugFile != nil {
		debugFile.WriteString(fmt.Sprintf("%s: %s\n", time.Now().Format("15:04:05"), msg))
		debugFile.Sync()
	}
}
