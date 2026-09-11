package download

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/tiredbooy/Rum/backend/internal/pkg/config"
)

func debugLogPath(t *testing.T) string {
	t.Helper()
	dir, err := os.UserConfigDir()
	if err != nil {
		t.Fatalf("UserConfigDir: %v", err)
	}
	return filepath.Join(dir, "rum", "logs", "debug.log")
}

// isolateConfig points the process at a throwaway config dir and guarantees the
// debug sink is closed again, so one test can never leak a handle into another.
func isolateConfig(t *testing.T) {
	t.Helper()
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	t.Cleanup(func() { _ = SetDebugLogging(false) })
	if err := SetDebugLogging(false); err != nil {
		t.Fatalf("reset debug logging: %v", err)
	}
}

// log_level=debug is what turns the verbose trace on. It used to be opened
// unconditionally, so the setting changed nothing and every user accumulated a
// debug.log forever.
func TestDebugLogWritesOnlyWhenEnabled(t *testing.T) {
	isolateConfig(t)

	DebugLog("must not be written")
	if _, err := os.Stat(debugLogPath(t)); !os.IsNotExist(err) {
		t.Fatal("debug.log was created while debug logging is off")
	}

	if err := SetDebugLogging(true); err != nil {
		t.Fatalf("enable: %v", err)
	}
	if !DebugLoggingEnabled() {
		t.Fatal("DebugLoggingEnabled() = false after enabling")
	}
	DebugLog("hello from the trace log")

	body, err := os.ReadFile(debugLogPath(t))
	if err != nil {
		t.Fatalf("read debug.log: %v", err)
	}
	if !strings.Contains(string(body), "hello from the trace log") {
		t.Fatalf("trace line missing from debug.log: %q", body)
	}
}

// Turning it back off must stop writing without losing what is already there.
func TestDebugLogStopsWhenDisabled(t *testing.T) {
	isolateConfig(t)

	if err := SetDebugLogging(true); err != nil {
		t.Fatalf("enable: %v", err)
	}
	DebugLog("first")
	if err := SetDebugLogging(false); err != nil {
		t.Fatalf("disable: %v", err)
	}
	DebugLog("second")

	body, err := os.ReadFile(debugLogPath(t))
	if err != nil {
		t.Fatalf("read debug.log: %v", err)
	}
	if !strings.Contains(string(body), "first") {
		t.Fatal("existing trace history was lost")
	}
	if strings.Contains(string(body), "second") {
		t.Fatal("kept writing after debug logging was turned off")
	}
	if DebugLoggingEnabled() {
		t.Fatal("DebugLoggingEnabled() = true after disabling")
	}
}

// Enabling twice must not leak the first handle or truncate the file.
func TestSetDebugLoggingIsIdempotent(t *testing.T) {
	isolateConfig(t)

	if err := SetDebugLogging(true); err != nil {
		t.Fatalf("enable: %v", err)
	}
	DebugLog("kept")
	if err := SetDebugLogging(true); err != nil {
		t.Fatalf("re-enable: %v", err)
	}
	DebugLog("also kept")

	body, err := os.ReadFile(debugLogPath(t))
	if err != nil {
		t.Fatalf("read debug.log: %v", err)
	}
	for _, want := range []string{"kept", "also kept"} {
		if !strings.Contains(string(body), want) {
			t.Fatalf("%q missing after re-enabling: %q", want, body)
		}
	}
}

// InitLogFile applies whatever the settings file says at startup.
func TestInitLogFileHonoursThePersistedLevel(t *testing.T) {
	isolateConfig(t)

	var s config.Setting
	s.OutDir = t.TempDir()
	s.LogLevel = "info"
	if err := s.Save(); err != nil {
		t.Fatalf("save: %v", err)
	}
	if err := InitLogFile(); err != nil {
		t.Fatalf("InitLogFile: %v", err)
	}
	if DebugLoggingEnabled() {
		t.Fatal("debug tracing on at level=info")
	}

	s.LogLevel = "debug"
	if err := s.Save(); err != nil {
		t.Fatalf("save: %v", err)
	}
	if err := InitLogFile(); err != nil {
		t.Fatalf("InitLogFile: %v", err)
	}
	if !DebugLoggingEnabled() {
		t.Fatal("debug tracing off at level=debug")
	}
}

// Changing the level in the settings UI must take effect without a restart —
// which is what lets the control avoid a "restart required" caveat.
func TestApplySettingsTogglesDebugLoggingLive(t *testing.T) {
	isolateConfig(t)
	m := NewJobManager(&Options{Parallel: 1})
	t.Cleanup(m.Shutdown)

	base := config.Setting{OutDir: t.TempDir(), MaxParallel: 1, Connections: 8}

	on := base
	on.LogLevel = "debug"
	m.ApplySettings(on)
	if !DebugLoggingEnabled() {
		t.Fatal("ApplySettings(debug) did not turn tracing on")
	}

	off := base
	off.LogLevel = "info"
	m.ApplySettings(off)
	if DebugLoggingEnabled() {
		t.Fatal("ApplySettings(info) did not turn tracing off")
	}
}
