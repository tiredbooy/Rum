package filesystem

import (
	"log"
	"os"
)

func CreateGroupFolder(folderPath string) {
	if folderPath == "" {
		return
	}

	_, err := os.Stat(folderPath)

	if os.IsNotExist(err) {
		err := os.MkdirAll(folderPath, 0755)
		if err != nil {
			log.Println("Failed to Create Directory")
			return
		}
	}

}
