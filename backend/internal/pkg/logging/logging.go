// Package logging is a thin, dependency-free wrapper over the standard library
// log/slog. It exposes a process-wide logger that is safe for concurrent use
// and safe to call before Init (it falls back to a default text logger at info
// level). Initialize it once at startup with Init.
package logging

import (
	"log/slog"
	"os"
	"strings"
	"sync/atomic"
)

// logger holds the active *slog.Logger. It is read/written atomically so the
// package is safe for concurrent use and never returns nil, even before Init.
var logger atomic.Pointer[slog.Logger]

func init() {
	// No-op-safe default: text handler at info level. Replaced by Init.
	logger.Store(newLogger(slog.LevelInfo, false))
}

// newLogger builds a *slog.Logger with the given level and handler kind.
func newLogger(level slog.Level, jsonOutput bool) *slog.Logger {
	opts := &slog.HandlerOptions{Level: level}
	var h slog.Handler
	if jsonOutput {
		h = slog.NewJSONHandler(os.Stderr, opts)
	} else {
		h = slog.NewTextHandler(os.Stderr, opts)
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
