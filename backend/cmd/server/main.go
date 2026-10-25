package server

import (
	"context"
	"errors"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strconv"
	"sync"
	"sync/atomic"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/tiredbooy/Rum/backend/internal/pkg/api/handlers"
	"github.com/tiredbooy/Rum/backend/internal/pkg/api/middlewares"
	"github.com/tiredbooy/Rum/backend/internal/pkg/api/routes"
	"github.com/tiredbooy/Rum/backend/internal/pkg/config"
	"github.com/tiredbooy/Rum/backend/internal/pkg/download"
	"github.com/tiredbooy/Rum/backend/internal/pkg/logging"
)

// InitLogging sends logs to a rotating file under <UserConfigDir>/rum/logs (in
// addition to stderr) and redirects the stdlib log package there too. In a
// packaged GUI build stderr is discarded, so this is the only way a user can
// retrieve logs for a bug report. Best-effort: on any error we keep logging to
// stderr. Exposed here (not in the internal logging package) so the root Wails
// main can call it without importing an internal/ package.
func InitLogging() {
	cfgDir, err := os.UserConfigDir()
	if err != nil {
		return
	}

	// Honour the persisted log_level. It was validated and round-tripped by the
	// settings store but the level here was hard-coded to "info", so switching to
	// "debug" for a bug report produced no extra output. A failed read just leaves
	// the level empty, which parseLevel maps to the info default.
	var setting config.Setting
	_ = setting.LoadSettingMetadata()

	w, path, err := logging.InitFileLogging(filepath.Join(cfgDir, "rum"), setting.LogLevel, false)
	if err != nil {
		return
	}
	log.SetOutput(w)
	log.Printf("rum starting; logs at %s (level=%s)", path, setting.LogLevel)
}

// preferredPort is the port the API tries first. If it is already in use the
// server falls back to an OS-assigned free port (see Listen) so a second Rum
// instance — or any process already holding 8080 — does not hard-fail startup.
const preferredPort = "8080"

// loopbackHost binds the API to the loopback interface only. The desktop
// frontend and the local CLI/TUI talk to it over 127.0.0.1; binding all
// interfaces would needlessly expose the download manager to the LAN.
const loopbackHost = "127.0.0.1"

var AppQuitFunc func()

// resumedOnLaunch is how many interrupted downloads the launch auto-resume
// restarted. The frontend is not mounted yet when that runs, so it fetches the
// count once via TakeResumedOnLaunch instead of listening for an event.
var resumedOnLaunch atomic.Int32

// TakeResumedOnLaunch returns the launch auto-resume count and resets it, so the
// "resumed N downloads" notice shows once per launch.
func TakeResumedOnLaunch() int { return int(resumedOnLaunch.Swap(0)) }

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
	// resumer implements the auto-resume-on-reconnect setting; stopped alongside
	// the schedule controller on graceful shutdown.
	resumer *download.ResumeController
	// diskWatch resumes downloads that stopped on a full disk once space is free.
	diskWatch *download.DiskWatcher
	// nightCtl applies the night plan (window start/stop and the data cap).
	nightCtl *download.NightController
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

	// Re-arm-per-session: never let a destructive post-download power action
	// (shutdown/sleep/close) survive a restart and fire on the next completion.
	if setting.DisarmDestructivePostDownload() {
		if err := setting.Save(); err != nil {
			log.Printf("could not disarm persisted post-download action: %v", err)
		}
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
		// Reliability / UX settings threaded into the engine: integrity
		// verification on completion, retry backoff base, temp dir for in-progress
		// files, and whether to keep partials on failure.
		VerifyIntegrity:      setting.VerifyIntegrity,
		RetryBackoffSec:      setting.RetryBackoffSec,
		TempDir:              setting.TempDir,
		KeepPartialOnFailure: setting.KeepPartialOnFailure,
		// Auto-organize master switch. finalizeCategorize gates on opt.Categorize,
		// so without this the enable_categories setting + its rules were persisted
		// and displayed but never moved a finished file anywhere.
		Categorize: setting.EnableCategories,
	}
	opt.Downloader = download.NewDownloader("", "")

	download.SetQuitFunc(AppQuitFunc)

	handlers.InitAPI(opt)

	// Auto-resume previously-running/paused jobs on launch when the setting is on.
	// This is the AutoResumeOnLaunch hook: a restart picks up where the last
	// session left off instead of leaving in-flight jobs stuck. AutoResumeOnReconnect
	// is realized by the engine retry/backoff policy (it resumes transient network
	// failures rather than failing); full OS network-event detection is a follow-up.
	if setting.AutoResumeOnLaunch && handlers.GlobalManager != nil {
		resumedOnLaunch.Store(int32(handlers.GlobalManager.ResumeInterruptedJobs(context.Background())))
	}

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
		resumer = download.NewResumeController(manager)
		diskWatch = download.NewDiskWatcher(manager)
		nightCtl = download.NewNightController(manager)
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
	res := resumer
	dw := diskWatch
	nc := nightCtl
	mgr := manager
	srvMu.Unlock()

	if s == nil || l == nil {
		log.Println("Serve called before Listen; nothing to do")
		return
	}

	// Long-lived context for background controllers. Cancelled on shutdown so
	// their ticker goroutines exit cleanly. Shared by the schedule controller and
	// the idle memory controller.
	rootCtx, cancel := context.WithCancel(context.Background())
	srvMu.Lock()
	rootCancel = cancel
	srvMu.Unlock()

	// Start the schedule controller (bandwidth windows + scheduled starts).
	if ctrl != nil {
		ctrl.Start(rootCtx)
	}

	// Start the reconnect watcher (auto_resume_on_reconnect). It re-queues
	// network-failed downloads once their host is reachable again.
	if res != nil {
		res.Start(rootCtx)
	}
	if dw != nil {
		dw.Start(rootCtx)
	}
	if nc != nil {
		nc.Start(rootCtx)
	}

	// Start the idle-gated memory controller so the runtime returns freed heap
	// pages to the OS when no downloads are active (Go releases memory lazily, so
	// RSS otherwise stays high after a large download). It self-stops when rootCtx
	// is cancelled. Started here because the root Wails module cannot import
	// backend/internal — the manager is only reachable from inside cmd/server.
	if mgr != nil {
		download.StartMemoryController(rootCtx, mgr.ActiveCount, 30*time.Second)
	}

	go func() {
		if err := s.Serve(l); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Fatalf("listen: %v", err)
		}
	}()
	log.Printf("Rum API server listening on %s", APIBase())

	// Periodic backstop: a crash or power loss mid-download then costs at most
	// a few seconds of progress metadata in jobs.json.
	if mgr != nil {
		go mgr.RunPeriodicPersist(rootCtx, persistInterval)
	}

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM, syscall.SIGHUP)
	<-quit
	log.Println("Shutting down server...")
	GracefulStop()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := s.Shutdown(ctx); err != nil {
		log.Printf("graceful shutdown failed: %v", err)
	}
}

// persistInterval is how often running jobs are flushed to jobs.json.
const persistInterval = 5 * time.Second

// quitDeadline bounds the whole pause-and-save sequence on quit.
const quitDeadline = 8 * time.Second

var stopOnce sync.Once

// GracefulStop pauses running downloads, saves their state and stops the
// background controllers, within quitDeadline. It is idempotent and shared by
// the signal path and the Wails shutdown hook, so whichever fires first does
// the work and the other returns immediately.
func GracefulStop() {
	stopOnce.Do(func() {
		srvMu.Lock()
		ctrl, res, mgr, cancel := controller, resumer, manager, rootCancel
		dw := diskWatch
		nc := nightCtl
		srvMu.Unlock()

		// Stop the background controllers first so none restarts a job while
		// the manager is pausing them.
		if ctrl != nil {
			ctrl.Stop()
		}
		if dw != nil {
			dw.Stop()
		}
		if nc != nil {
			nc.Stop()
		}
		if res != nil {
			res.Stop()
		}
		if cancel != nil {
			cancel()
		}
		if mgr != nil {
			ctx, done := context.WithTimeout(context.Background(), quitDeadline)
			defer done()
			mgr.GracefulShutdown(ctx)
		}
	})
}

// ActiveDownloads reports how many downloads are running right now, so the
// desktop layer can warn before quitting.
func ActiveDownloads() int {
	srvMu.Lock()
	mgr := manager
	srvMu.Unlock()
	if mgr == nil {
		return 0
	}
	return mgr.ActiveCount()
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
