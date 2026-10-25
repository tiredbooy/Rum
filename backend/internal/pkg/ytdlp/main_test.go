package ytdlp

import (
	"os"
	"testing"

	"github.com/tiredbooy/Rum/backend/internal/pkg/testenv"
	"github.com/tiredbooy/Rum/backend/internal/pkg/ytdlp/fakeytdlp"
)

// TestMain doubles as the fake yt-dlp: tests run this very binary as the tool.
func TestMain(m *testing.M) {
	if os.Getenv(fakeytdlp.Env) != "" {
		os.Exit(fakeytdlp.Main(os.Args[1:], os.Stdout, os.Stderr))
	}
	os.Exit(testenv.Run(m))
}

// fakeBin points the code under test at this test binary acting as yt-dlp.
func fakeBin(t *testing.T, mode string) string {
	t.Helper()
	t.Setenv(fakeytdlp.Env, "1")
	t.Setenv(fakeytdlp.ModeEnv, mode)
	return os.Args[0]
}
