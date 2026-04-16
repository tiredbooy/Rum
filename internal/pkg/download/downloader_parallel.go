package download

import (
	"log"
)

func DownloadWorker(opt Options) {
	log.Println("URLwww: ", opt.URL)
	if opt.URL == "" {
		log.Println("You need to Provide a Valid url")
		return
	}

	err := StartDownload(opt)
	if err != nil {
		log.Println("Error While Downloading...")
		return
	}
}
