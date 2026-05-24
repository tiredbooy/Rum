package server

import (
	"log"

	"github.com/gin-gonic/gin"
	"github.com/tiredbooy/Rum/backend/internal/pkg/api/handlers"
	"github.com/tiredbooy/Rum/backend/internal/pkg/api/middlewares"
	"github.com/tiredbooy/Rum/backend/internal/pkg/api/routes"
	"github.com/tiredbooy/Rum/backend/internal/pkg/config"
	"github.com/tiredbooy/Rum/backend/internal/pkg/download"
)

var AppQuitFunc func()

func SetQuitFunc(f func()) {
	AppQuitFunc = f
}

func Start() {
	download.InitLogFile()
	
	var setting config.Setting
	err := setting.LoadSettingMetadata()
	if err != nil {
		log.Println("Error Opening setting: ", err.Error())
		return
	}


	if setting.SpeedLimitKB < 0 {
		setting.SpeedLimitKB = 0
	}

	opt := &download.Options{
		SpeedLimit: setting.SpeedLimitKB,
		Parallel:   setting.MaxParallel,
		Out:        setting.OutDir,
		MaxRetries: setting.MaxRetries,
		Silent:     setting.Silent,
	}
	opt.Downloader = download.NewDownloader("", "")

	download.SetQuitFunc(AppQuitFunc)

	handlers.InitAPI(opt)

	r := gin.Default()
	middlewares.SetupMiddlewares(r)
	routes.SetupRouter(r)

	r.Run(":8080")
}

// func main() {
// 	Start()
// }
