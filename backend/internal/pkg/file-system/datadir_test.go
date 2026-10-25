package filesystem

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestDataDirHonoursEnvOverride(t *testing.T) {
	dir := t.TempDir()
	t.Setenv(EnvDataDir, dir)
	if got := DataDir(); got != dir {
		t.Fatalf("DataDir() = %q, want %q", got, dir)
	}
	if got := CreateMetadataFile("jobs.json"); got != filepath.Join(dir, "jobs.json") {
		t.Fatalf("CreateMetadataFile = %q", got)
	}
}

func TestDataDirFallsBackToUserConfigDir(t *testing.T) {
	xdg := t.TempDir()
	t.Setenv(EnvDataDir, "")
	t.Setenv("XDG_CONFIG_HOME", xdg)
	if got, want := DataDir(), filepath.Join(xdg, "rum"); got != want {
		t.Fatalf("DataDir() = %q, want %q", got, want)
	}
}

func TestDataDirPanicsOnRealUserDirUnderTest(t *testing.T) {
	t.Setenv(EnvDataDir, realUserDataDir)
	defer func() {
		r := recover()
		if r == nil {
			t.Fatal("expected panic when data dir is the real user dir")
		}
		if msg, _ := r.(string); !strings.Contains(msg, EnvDataDir) {
			t.Fatalf("panic message should mention %s, got %v", EnvDataDir, r)
		}
	}()
	DataDir()
}
