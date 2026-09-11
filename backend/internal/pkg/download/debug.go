package download

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/tiredbooy/Rum/backend/internal/pkg/config"
)

// debugFile is the verbose per-download trace sink (<config>/rum/logs/debug.log).
// nil means debug logging is off and DebugLog is a no-op.
//
// It is guarded by debugMu because the settings API can open or close it while
// download goroutines are writing to it. DebugLog fires a handful of times per
// download, so a plain mutex costs nothing and is far easier to reason about
// than swapping a file handle out from under a concurrent Write.
var (
	debugMu   sync.Mutex
	debugFile *os.File
)

// debugLogLevel is the config log_level value that turns verbose tracing on.
const debugLogLevel = "debug"

// InitLogFile applies the persisted log_level to the debug trace log at startup.
//
// This file used to be opened unconditionally, so every user accumulated a
// debug.log forever and the log_level setting — which was validated, persisted
// and round-tripped — changed nothing at all. Now "debug" opens it and any other
// level leaves it closed.
func InitLogFile() error {
	var setting config.Setting
	_ = setting.LoadSettingMetadata() // a failed read leaves the level empty => off
	return SetDebugLogging(setting.LogLevel == debugLogLevel)
}

// SetDebugLogging opens or closes the verbose trace log, so switching log level
// in the settings UI takes effect immediately instead of at the next launch.
// Calling it with the state it is already in is a no-op.
func SetDebugLogging(enabled bool) error {
	debugMu.Lock()
	defer debugMu.Unlock()

	if !enabled {
		if debugFile != nil {
			_ = debugFile.Close()
			debugFile = nil
		}
		return nil
	}
	if debugFile != nil {
		return nil // already on
	}

	path, err := os.UserConfigDir()
	if err != nil {
		log.Println("Failed to get user config path")
		return err
	}
	dir := filepath.Join(path, "rum", "logs")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}

	// Append (not truncate) so debug history survives restarts.
	f, err := os.OpenFile(filepath.Join(dir, "debug.log"), os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		return err
	}
	debugFile = f
	return nil
}

// DebugLoggingEnabled reports whether verbose tracing is currently on.
func DebugLoggingEnabled() bool {
	debugMu.Lock()
	defer debugMu.Unlock()
	return debugFile != nil
}

func DebugLog(msg string) {
	debugMu.Lock()
	defer debugMu.Unlock()
	if debugFile == nil {
		return
	}
	fmt.Fprintf(debugFile, "%s: %s\n", time.Now().Format("15:04:05"), msg)
	_ = debugFile.Sync()
}
