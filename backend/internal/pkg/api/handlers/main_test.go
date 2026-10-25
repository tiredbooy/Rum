package handlers

import (
	"os"
	"testing"

	"github.com/tiredbooy/Rum/backend/internal/pkg/download"

	"github.com/tiredbooy/Rum/backend/internal/pkg/testenv"
)

// TestMain keeps every test off the real user config dir.
func TestMain(m *testing.M) { os.Exit(testenv.Run(m)) }

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
