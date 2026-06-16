package logging

import (
	"bytes"
	"log/slog"
	"strings"
	"sync"
	"testing"
)

func TestParseLevel(t *testing.T) {
	tests := []struct {
		in   string
		want slog.Level
	}{
		{"debug", slog.LevelDebug},
		{"DEBUG", slog.LevelDebug},
		{"info", slog.LevelInfo},
		{" Info ", slog.LevelInfo},
		{"warn", slog.LevelWarn},
		{"warning", slog.LevelWarn},
		{"error", slog.LevelError},
		{"", slog.LevelInfo},
		{"nonsense", slog.LevelInfo},
	}
	for _, tc := range tests {
		if got := parseLevel(tc.in); got != tc.want {
			t.Errorf("parseLevel(%q) = %v, want %v", tc.in, got, tc.want)
		}
	}
}

func TestInitAndL(t *testing.T) {
	// L must be non-nil before any Init (init() default) and after Init.
	if L() == nil {
		t.Fatal("L() returned nil before Init")
	}
	Init("debug", true)
	if L() == nil {
		t.Fatal("L() returned nil after Init")
	}
}

func TestConvenienceHelpersNoPanic(t *testing.T) {
	// Should not panic at any level, including before/after Init.
	Init("info", false)
	Debug("debug msg", "k", "v")
	Info("info msg", "k", "v")
	Warn("warn msg", "k", "v")
	Error("error msg", "k", "v")
}

func TestLevelFiltering(t *testing.T) {
	// Verify the configured level actually filters output by swapping in a
	// logger backed by a buffer at warn level.
	var buf bytes.Buffer
	logger.Store(slog.New(slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: slog.LevelWarn})))

	Info("should be filtered")
	if buf.Len() != 0 {
		t.Errorf("info logged at warn level: %q", buf.String())
	}
	Warn("should appear")
	if !strings.Contains(buf.String(), "should appear") {
		t.Errorf("warn not logged at warn level: %q", buf.String())
	}
}

func TestConcurrentSafe(t *testing.T) {
	var wg sync.WaitGroup
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			Init("info", false)
			Info("concurrent", "i", 1)
			_ = L()
		}()
	}
	wg.Wait()
}
