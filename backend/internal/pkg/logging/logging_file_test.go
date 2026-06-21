package logging

import (
	"log"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestInitFileLogging guards C5: GUI logs must land in a retrievable file, not
// just stderr (which is discarded in a packaged build). Both the stdlib log
// redirect and slog must reach the file.
func TestInitFileLogging(t *testing.T) {
	defer logger.Store(newLogger(slog.LevelInfo, false)) // restore global logger

	dir := t.TempDir()
	w, path, err := InitFileLogging(dir, "info", false)
	if err != nil {
		t.Fatalf("InitFileLogging: %v", err)
	}
	if got := filepath.Dir(path); got != filepath.Join(dir, "logs") {
		t.Fatalf("log path %s not under %s/logs", path, dir)
	}

	log.SetOutput(w)
	defer log.SetOutput(os.Stderr)
	log.Println("hello-from-stdlib")
	Info("hello-from-slog", "k", "v")

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read log file: %v", err)
	}
	s := string(data)
	if !strings.Contains(s, "hello-from-stdlib") || !strings.Contains(s, "hello-from-slog") {
		t.Fatalf("log file is missing expected entries:\n%s", s)
	}
}
