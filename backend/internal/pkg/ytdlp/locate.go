package ytdlp

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"

	filesystem "github.com/tiredbooy/Rum/backend/internal/pkg/file-system"
)

// Where a located binary came from.
const (
	SourceConfigured = "configured"
	SourcePath       = "path"
	SourceManaged    = "managed" // installed by Rum into its tools dir
)

// Found is a usable yt-dlp binary.
type Found struct {
	Path   string
	Source string
}

// ToolsDir is where Rum keeps tools it installed itself.
func ToolsDir() string { return filepath.Join(filesystem.DataDir(), "tools") }

// BinaryName is the file name of yt-dlp on goos.
func BinaryName(goos string) string {
	if goos == "windows" {
		return "yt-dlp.exe"
	}
	return "yt-dlp"
}

// ManagedPath is where Rum installs yt-dlp.
func ManagedPath() string { return filepath.Join(ToolsDir(), BinaryName(runtime.GOOS)) }

func isFile(p string) bool {
	st, err := os.Stat(p)
	return err == nil && st.Mode().IsRegular()
}

// Locate finds yt-dlp: the configured path first, then PATH, then Rum's tools
// dir. It returns a ytdlp_missing error when none is usable.
func Locate(configured string) (Found, error) {
	if configured != "" && isFile(configured) {
		return Found{Path: configured, Source: SourceConfigured}, nil
	}
	if p, err := exec.LookPath(BinaryName(runtime.GOOS)); err == nil {
		return Found{Path: p, Source: SourcePath}, nil
	}
	if p := ManagedPath(); isFile(p) {
		return Found{Path: p, Source: SourceManaged}, nil
	}
	return Found{}, missingError()
}

// HasFFmpeg reports whether ffmpeg is on PATH. Without it yt-dlp cannot merge
// separate video and audio streams, so only single-file formats are offered.
func HasFFmpeg() bool {
	_, err := exec.LookPath("ffmpeg")
	return err == nil
}
