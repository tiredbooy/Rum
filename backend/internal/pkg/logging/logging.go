// Package logging is a thin, dependency-free wrapper over the standard library
// log/slog. It exposes a process-wide logger that is safe for concurrent use
// and safe to call before Init (it falls back to a default text logger at info
// level). Initialize it once at startup with Init.
package logging

import (
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
)

// maxLogBytes is the size at which rum.log is rotated to rum.log.1 on startup.
const maxLogBytes = 5 << 20 // 5 MiB

// logger holds the active *slog.Logger. It is read/written atomically so the
// package is safe for concurrent use and never returns nil, even before Init.
var logger atomic.Pointer[slog.Logger]

func init() {
	// No-op-safe default: text handler at info level. Replaced by Init.
	logger.Store(newLogger(slog.LevelInfo, false))
}

// newLogger builds a *slog.Logger writing to stderr (the default sink).
func newLogger(level slog.Level, jsonOutput bool) *slog.Logger {
	return newLoggerWithWriter(os.Stderr, level, jsonOutput)
}

// newLoggerWithWriter builds a *slog.Logger writing to w.
func newLoggerWithWriter(w io.Writer, level slog.Level, jsonOutput bool) *slog.Logger {
	opts := &slog.HandlerOptions{Level: level}
	var h slog.Handler
	if jsonOutput {
		h = slog.NewJSONHandler(w, opts)
	} else {
		h = slog.NewTextHandler(w, opts)
	}
	return slog.New(h)
}

// parseLevel maps a textual level to an slog.Level. Unknown/empty values
// default to info. Parsing is case-insensitive and tolerant of surrounding
// whitespace.
func parseLevel(level string) slog.Level {
	switch strings.ToLower(strings.TrimSpace(level)) {
	case "debug":
		return slog.LevelDebug
	case "warn", "warning":
		return slog.LevelWarn
	case "error":
		return slog.LevelError
	case "info":
		return slog.LevelInfo
	default:
		return slog.LevelInfo
	}
}

// Init configures the process-wide logger. level is one of
// debug/info/warn/error (case-insensitive; unknown -> info). If jsonOutput is
// true a JSON handler is used, otherwise a text handler. Safe to call multiple
// times and safe for concurrent use with the convenience helpers.
func Init(level string, jsonOutput bool) {
	logger.Store(newLogger(parseLevel(level), jsonOutput))
}

// InitFileLogging points the process-wide logger at a rotating file under
// dir/logs in addition to stderr, and returns the combined writer (so callers
// can also redirect the stdlib `log` package to it) and the log file path.
//
// In a packaged GUI build stderr goes nowhere, so without a file sink there is
// no way for a user to retrieve logs for a bug report — this is what makes that
// possible. Rotation is a simple size check at startup (rum.log -> rum.log.1),
// which needs no third-party dependency.
func InitFileLogging(dir, level string, jsonOutput bool) (io.Writer, string, error) {
	logDir := filepath.Join(dir, "logs")
	if err := os.MkdirAll(logDir, 0o755); err != nil {
		return nil, "", err
	}
	path := filepath.Join(logDir, "rum.log")
	rotateIfLarge(path, maxLogBytes)
	f, err := os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		return nil, "", err
	}
	w := io.MultiWriter(os.Stderr, f)
	logger.Store(newLoggerWithWriter(w, parseLevel(level), jsonOutput))
	return w, path, nil
}

// rotateIfLarge renames path to path.1 when it exceeds max bytes, so the live
// log never grows unbounded. Best-effort: any error is ignored (logging must
// never block startup).
func rotateIfLarge(path string, max int64) {
	info, err := os.Stat(path)
	if err != nil || info.Size() < max {
		return
	}
	_ = os.Rename(path, path+".1")
}

// L returns the active logger. It never returns nil.
func L() *slog.Logger {
	return logger.Load()
}

// Debug logs at debug level via the active logger.
func Debug(msg string, args ...any) { L().Debug(msg, args...) }

// Info logs at info level via the active logger.
func Info(msg string, args ...any) { L().Info(msg, args...) }

// Warn logs at warn level via the active logger.
func Warn(msg string, args ...any) { L().Warn(msg, args...) }

// Error logs at error level via the active logger.
func Error(msg string, args ...any) { L().Error(msg, args...) }
