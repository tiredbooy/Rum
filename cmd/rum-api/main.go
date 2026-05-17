package main

import (
	"github.com/gin-gonic/gin"
	"swiftget.com/internal/pkg/api/handlers"
	"swiftget.com/internal/pkg/api/middlewares"
	"swiftget.com/internal/pkg/api/routes"
	"swiftget.com/internal/pkg/download"
	filesystem "swiftget.com/internal/pkg/file-system"
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
