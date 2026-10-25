package download

import (
	"os"
	"testing"

	"github.com/tiredbooy/Rum/backend/internal/pkg/testenv"
	"github.com/tiredbooy/Rum/backend/internal/pkg/ytdlp/fakeytdlp"
)

// TestMain keeps every test off the real user config dir. It also doubles as the
// fake yt-dlp for the video tests (see fakeytdlp): the test binary is run as
// the tool when RUM_FAKE_YTDLP is set.
func TestMain(m *testing.M) {
	if os.Getenv(fakeytdlp.Env) != "" {
		os.Exit(fakeytdlp.Main(os.Args[1:], os.Stdout, os.Stderr))
	}
	os.Exit(testenv.Run(m))
}
