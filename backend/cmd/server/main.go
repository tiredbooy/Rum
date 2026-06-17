package server

import (
	"context"
	"errors"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"sync"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/tiredbooy/Rum/backend/internal/pkg/api/handlers"
	"github.com/tiredbooy/Rum/backend/internal/pkg/api/middlewares"
	"github.com/tiredbooy/Rum/backend/internal/pkg/api/routes"
	"github.com/tiredbooy/Rum/backend/internal/pkg/config"
	"github.com/tiredbooy/Rum/backend/internal/pkg/download"
)

// preferredPort is the port the API tries first. If it is already in use the
// server falls back to an OS-assigned free port (see Listen) so a second Rum
// instance — or any process already holding 8080 — does not hard-fail startup.
const preferredPort = "8080"

// loopbackHost binds the API to the loopback interface only. The desktop
// frontend and the local CLI/TUI talk to it over 127.0.0.1; binding all
// interfaces would needlessly expose the download manager to the LAN.
const loopbackHost = "127.0.0.1"

var AppQuitFunc func()

func SetQuitFunc(f func()) {
	AppQuitFunc = f
}

// Package-level server state, populated by Listen and consumed by Serve /
// APIBase. Guarded by srvMu so concurrent Listen/APIBase access from the desktop
// layer is race-free.
var (
	srvMu   sync.Mutex
	srv     *http.Server
	ln      net.Listener
	apiBase string

	// controller, when set in Listen, is the bandwidth/scheduled-start ticker; it
	// is stopped on graceful shutdown.
	controller *download.ScheduleController
	// manager is the live job manager (handlers.GlobalManager) captured at Listen
	// time so Serve can shut its dispatcher down cleanly.
	manager *download.JobManager
	// rootCancel cancels the controller's context on shutdown.
	rootCancel context.CancelFunc
)

// Listen builds the API router, binds it to the loopback interface, and records
// the resolved base URL. It prefers preferredPort and falls back to an
// OS-assigned free port (":0") when the preferred one is taken, so startup never
// hard-fails on a busy port.
//
// It returns the resolved base URL (e.g. "http://127.0.0.1:8080"). The server is
// not yet accepting connections when Listen returns — call Serve (typically in a
// goroutine) to start handling requests. This split lets the desktop layer learn
// the real port (via APIBase) before any frontend code runs.
func Listen() (baseURL string, err error) {
	download.InitLogFile()

	var setting config.Setting
	if err := setting.LoadSettingMetadata(); err != nil {
		log.Println("Error Opening setting: ", err.Error())
		// Continue with whatever defaults LoadSettingMetadata applied rather than
		// refusing to start the server.
	}

	if setting.SpeedLimitKB < 0 {
		setting.SpeedLimitKB = 0
	}

	// Build a shared speed governor from the persisted limit. The schedule
	// controller (started in Serve) updates it live as bandwidth windows open and
	// close; the engine reads it per Read so changes apply mid-stream. Engine/CLI
	// callers that don't supply a governor keep the per-download limiter.
	governor := download.NewSpeedGovernor(setting.SpeedLimitKB)

	opt := &download.Options{
		SpeedLimit:  setting.SpeedLimitKB,
		Parallel:    setting.MaxParallel,
		Connections: setting.Connections,
		Out:         setting.OutDir,
		MaxRetries:  setting.MaxRetries,
		Silent:      setting.Silent,
		Governor:    governor,
	}
	opt.Downloader = download.NewDownloader("", "")

	download.SetQuitFunc(AppQuitFunc)

	handlers.InitAPI(opt)

	r := gin.Default()
	middlewares.SetupMiddlewares(r)
	routes.SetupRouter(r)

	// Bind the listener up front so we can resolve the real port before serving.
	listener, addr, err := bindLoopback()
	if err != nil {
		return "", err
	}

	srvMu.Lock()
	ln = listener
	apiBase = "http://" + addr
	srv = &http.Server{
		Handler:           r,
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       15 * time.Second,
		// WriteTimeout intentionally 0: the SSE progress streams are long-lived
		// and a write deadline would sever them.
		WriteTimeout: 0,
		IdleTimeout:  60 * time.Second,
	}

	// Wire the bandwidth/scheduled-start controller onto the live manager. It is
	// started in Serve and stopped on graceful shutdown so there is no leaked
	// ticker goroutine.
	manager = handlers.GlobalManager
	if manager != nil {
		controller = download.NewScheduleController(manager, governor, setting)
	}
	base := apiBase
	srvMu.Unlock()

	log.Printf("Rum API server bound on %s", base)
	return base, nil
}

// portScanSpan is how many sequential ports starting at preferredPort bindLoopback
// tries before asking the OS for any free port. Walking 8080..8089 first keeps the
// API on a predictable, low port across restarts (nicer for logs/debugging); the
// :0 fallback then guarantees startup never hard-fails even if all of them are
// busy.
const portScanSpan = 10

// bindLoopback opens a TCP listener on the loopback interface. It tries
// preferredPort first, then the next few sequential ports, and finally an
// OS-assigned free port (":0"), so a port already in use (a second Rum instance,
// or any other process holding 8080) never hard-fails startup. It returns the
// listener and the host:port string it is actually listening on.
func bindLoopback() (net.Listener, string, error) {
	return bindLoopbackScan(loopbackHost, preferredPort, portScanSpan)
}

// bindLoopbackScan tries startPort, then the next span-1 sequential ports on host,
// and finally an OS-assigned free port (":0"). It is split out from bindLoopback
// so a test can drive the sequential-fallback path deterministically against a
// known-busy port, instead of depending on whether 8080 happens to be free.
func bindLoopbackScan(host, startPort string, span int) (net.Listener, string, error) {
	first := net.JoinHostPort(host, startPort)

	if base, err := strconv.Atoi(startPort); err == nil {
		// Try startPort, then startPort+1 .. startPort+span-1.
		for i := 0; i < span; i++ {
			addr := net.JoinHostPort(host, strconv.Itoa(base+i))
			listener, err := net.Listen("tcp", addr)
			if err == nil {
				if i > 0 {
					log.Printf("port %s busy; bound on %s instead", first, listener.Addr().String())
				}
				return listener, listener.Addr().String(), nil
			}
		}
		log.Printf("ports %d-%d on %s all busy; falling back to a dynamic port", base, base+span-1, host)
	}

	// Every candidate port was taken (or startPort wasn't numeric): let the OS hand
	// us any free port on loopback.
	fallback := net.JoinHostPort(host, "0")
	listener, err := net.Listen("tcp", fallback)
	if err != nil {
		return nil, "", err
	}
	return listener, listener.Addr().String(), nil
}

// APIBase returns the resolved base URL recorded by Listen (e.g.
// "http://127.0.0.1:8080"). It returns "" before Listen has been called.
func APIBase() string {
	srvMu.Lock()
	defer srvMu.Unlock()
	return apiBase
}

// Serve starts handling requests on the listener opened by Listen and blocks
// until a termination signal arrives, then shuts the server down gracefully.
// Call it in a goroutine (the desktop layer does `go server.Serve()`). It is a
// no-op if Listen has not been called.
func Serve() {
	srvMu.Lock()
	s := srv
	l := ln
	ctrl := controller
	mgr := manager
	srvMu.Unlock()

	if s == nil || l == nil {
		log.Println("Serve called before Listen; nothing to do")
		return
	}

	// Start the schedule controller (bandwidth windows + scheduled starts). Its
	// context is cancelled on shutdown so the ticker goroutine exits cleanly.
	if ctrl != nil {
		ctx, cancel := context.WithCancel(context.Background())
		srvMu.Lock()
		rootCancel = cancel
		srvMu.Unlock()
		ctrl.Start(ctx)
	}

	go func() {
		if err := s.Serve(l); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Fatalf("listen: %v", err)
		}
	}()
	log.Printf("Rum API server listening on %s", APIBase())

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	log.Println("Shutting down server...")

	// Stop the controller and the manager dispatcher before closing the HTTP
	// server so no new downloads are scheduled mid-shutdown.
	if ctrl != nil {
		ctrl.Stop()
	}
	srvMu.Lock()
	if rootCancel != nil {
		rootCancel()
	}
	srvMu.Unlock()
	if mgr != nil {
		mgr.Shutdown()
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := s.Shutdown(ctx); err != nil {
		log.Printf("graceful shutdown failed: %v", err)
	}
}

// Start preserves the original entry point: it binds the server (Listen) and
// then serves it (Serve), blocking until shutdown. The root Wails module still
// calls `go server.Start()`, so keeping this wrapper avoids breaking it.
func Start() {
	if _, err := Listen(); err != nil {
		log.Printf("failed to bind API server: %v", err)
		return
	}
	Serve()
}
