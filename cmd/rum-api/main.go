package main

import (
	"github.com/gin-gonic/gin"
	"github.com/tiredbooy/Rum/internal/pkg/api/handlers"
	"github.com/tiredbooy/Rum/internal/pkg/api/middlewares"
	"github.com/tiredbooy/Rum/internal/pkg/api/routes"
	"github.com/tiredbooy/Rum/internal/pkg/download"
	filesystem "github.com/tiredbooy/Rum/internal/pkg/file-system"
)

func main() {
	// cfg := config.Load()
	download.InitLogFile()

	opt := &download.Options{
		Parallel:   1,
		Out:        filesystem.GetOrCreateDirectory(),
		MaxRetries: 3,
		Silent:     false,
	}
	opt.Downloader = download.NewDownloader("", "")

	handlers.InitAPI(opt)

	r := gin.Default()
	middlewares.SetupMiddlewares(r)
	routes.SetupRouter(r)

	r.Run(":8080")
}
