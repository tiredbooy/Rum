package server

import (
	"context"
	"errors"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

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

	srv := &http.Server{
		Addr:              ":8080",
		Handler:           r,
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       15 * time.Second,
		// WriteTimeout intentionally 0: the SSE progress streams are long-lived
		// and a write deadline would sever them.
		WriteTimeout: 0,
		IdleTimeout:  60 * time.Second,
	}

	go func() {
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Fatalf("listen: %v", err)
		}
	}()
	log.Println("Rum API server listening on :8080")

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	log.Println("Shutting down server...")

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := srv.Shutdown(ctx); err != nil {
		log.Printf("graceful shutdown failed: %v", err)
	}
}

// func main() {
// 	Start()
// }
