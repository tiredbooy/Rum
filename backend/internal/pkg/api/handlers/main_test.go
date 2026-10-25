package handlers

import (
	"os"
	"testing"

	"github.com/tiredbooy/Rum/backend/internal/pkg/download"

	"github.com/tiredbooy/Rum/backend/internal/pkg/testenv"
	"github.com/tiredbooy/Rum/backend/internal/pkg/ytdlp/fakeytdlp"
)

// TestMain keeps every test off the real user config dir. It also doubles as the
// fake yt-dlp for the video endpoint tests (see fakeytdlp).
func TestMain(m *testing.M) {
	if os.Getenv(fakeytdlp.Env) != "" {
		os.Exit(fakeytdlp.Main(os.Args[1:], os.Stdout, os.Stderr))
	}
	os.Exit(testenv.Run(m))
}

// freshManager points the data dir at a per-test temp dir and installs a new
// GlobalManager for the test, shutting it down and clearing it afterwards so no
// state leaks into the next test.
func freshManager(t *testing.T, opt *download.Options) {
	t.Helper()
	t.Setenv("RUM_CONFIG_DIR", t.TempDir())
	InitAPI(opt)
	t.Cleanup(func() {
		GlobalManager.Shutdown()
		GlobalManager = nil
	})
}
