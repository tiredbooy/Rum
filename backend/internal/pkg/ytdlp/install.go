package ytdlp

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

// ReleaseBase is the official "latest release" download location.
const ReleaseBase = "https://github.com/yt-dlp/yt-dlp/releases/latest/download"

const (
	sumsName     = "SHA2-256SUMS"
	maxBinary    = 200 << 20 // the largest release asset is ~40 MB
	maxSums      = 1 << 20
	installLimit = 10 * time.Minute
)

// AssetName is the release asset for an OS/arch, or "" when unsupported.
func AssetName(goos, goarch string) string {
	switch goos {
	case "linux":
		switch goarch {
		case "amd64":
			return "yt-dlp_linux"
		case "arm64":
			return "yt-dlp_linux_aarch64"
		case "arm":
			return "yt-dlp_linux_armv7l"
		}
	case "windows":
		switch goarch {
		case "amd64":
			return "yt-dlp.exe"
		case "386":
			return "yt-dlp_x86.exe"
		case "arm64":
			return "yt-dlp_arm64.exe"
		}
	case "darwin":
		return "yt-dlp_macos"
	}
	return ""
}

// Installer downloads the release binary. The zero value installs into
// ToolsDir for the running OS from the official release, using Client.
type Installer struct {
	Client  *http.Client // should carry Rum's proxy settings
	BaseURL string       // default ReleaseBase
	GOOS    string       // default runtime.GOOS
	GOARCH  string       // default runtime.GOARCH
	Dir     string       // default ToolsDir()
}

// ParseSums finds name's SHA-256 in a "<hex>  <file>" checksum list.
func ParseSums(sums, name string) (string, bool) {
	for _, line := range strings.Split(sums, "\n") {
		f := strings.Fields(line)
		if len(f) == 2 && strings.TrimPrefix(f[1], "*") == name && len(f[0]) == 64 {
			return strings.ToLower(f[0]), true
		}
	}
	return "", false
}

// Supported reports whether an official build exists for this system.
func (in Installer) Supported() bool {
	goos, goarch := in.GOOS, in.GOARCH
	if goos == "" {
		goos = runtime.GOOS
	}
	if goarch == "" {
		goarch = runtime.GOARCH
	}
	return AssetName(goos, goarch) != ""
}

func installError(msg string) *Error { return &Error{Code: CodeFailed, Message: msg, Retryable: true} }

// Install downloads the binary, verifies it against the release's SHA2-256SUMS
// and stores it executable. Nothing is left at the final path unless the
// checksum matched. It returns the installed path.
func (in Installer) Install(ctx context.Context) (string, error) {
	goos, goarch := in.GOOS, in.GOARCH
	if goos == "" {
		goos = runtime.GOOS
	}
	if goarch == "" {
		goarch = runtime.GOARCH
	}
	asset := AssetName(goos, goarch)
	if asset == "" {
		return "", &Error{Code: CodeFailed, Message: "No yt-dlp build for this system"}
	}
	base := strings.TrimRight(in.BaseURL, "/")
	if base == "" {
		base = ReleaseBase
	}
	dir := in.Dir
	if dir == "" {
		dir = ToolsDir()
	}
	client := in.Client
	if client == nil {
		client = http.DefaultClient
	}
	ctx, cancel := context.WithTimeout(ctx, installLimit)
	defer cancel()

	sumsBody, err := fetch(ctx, client, base+"/"+sumsName, maxSums)
	if err != nil {
		return "", wrapFetch(err, "Couldn't fetch the checksum list")
	}
	want, ok := ParseSums(string(sumsBody), asset)
	if !ok {
		return "", installError("Checksum list has no entry for this build")
	}

	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", &Error{Code: "permission", Message: "Can't write to the tools folder"}
	}
	tmp, err := os.CreateTemp(dir, ".yt-dlp-*.tmp")
	if err != nil {
		return "", &Error{Code: "permission", Message: "Can't write to the tools folder"}
	}
	tmpPath := tmp.Name()
	defer os.Remove(tmpPath) // no-op once renamed

	got, err := fetchTo(ctx, client, base+"/"+asset, tmp)
	if cerr := tmp.Close(); err == nil {
		err = cerr
	}
	if err != nil {
		return "", wrapFetch(err, "Couldn't download yt-dlp")
	}
	if got != want {
		return "", &Error{Code: CodeFailed, Message: "Download failed its checksum; try again"}
	}
	if err := os.Chmod(tmpPath, 0o755); err != nil {
		return "", installError("Couldn't make yt-dlp runnable")
	}
	final := filepath.Join(dir, BinaryName(goos))
	if err := replaceFile(tmpPath, final); err != nil {
		return "", &Error{Code: "permission", Message: "Can't write to the tools folder"}
	}
	return final, nil
}

// replaceFile renames src over dst. Windows refuses to rename over a file that
// exists, so it is removed first there.
func replaceFile(src, dst string) error {
	if err := os.Rename(src, dst); err == nil {
		return nil
	}
	_ = os.Remove(dst)
	return os.Rename(src, dst)
}

func wrapFetch(err error, msg string) error {
	if errors.Is(err, context.Canceled) {
		return err
	}
	e := ClassifyStderr(err.Error())
	if e.Code == CodeFailed {
		return installError(msg)
	}
	return e
}

func get(ctx context.Context, c *http.Client, url string) (*http.Response, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "Rum")
	resp, err := c.Do(req)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		resp.Body.Close()
		return nil, fmt.Errorf("http %d", resp.StatusCode)
	}
	return resp, nil
}

func fetch(ctx context.Context, c *http.Client, url string, limit int64) ([]byte, error) {
	resp, err := get(ctx, c, url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	return io.ReadAll(io.LimitReader(resp.Body, limit))
}

// fetchTo streams url into w and returns the hex SHA-256 of what was written.
func fetchTo(ctx context.Context, c *http.Client, url string, w io.Writer) (string, error) {
	resp, err := get(ctx, c, url)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	h := sha256.New()
	n, err := io.Copy(io.MultiWriter(w, h), io.LimitReader(resp.Body, maxBinary+1))
	if err != nil {
		return "", err
	}
	if n > maxBinary {
		return "", errors.New("download too large")
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}
