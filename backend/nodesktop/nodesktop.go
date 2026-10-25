// Package nodesktop is the single switch that stops Rum from touching the
// user's desktop (file managers, notifications, power actions, sleep
// inhibition, tray) while tests run or when RUM_NO_DESKTOP=1 is set.
package nodesktop

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// ErrBlocked is returned instead of performing a real desktop side effect.
var ErrBlocked = errors.New("desktop side effects are disabled (test run or RUM_NO_DESKTOP=1)")

// Blocked reports whether real desktop side effects must be refused.
func Blocked() bool {
	return testing.Testing() || os.Getenv("RUM_NO_DESKTOP") == "1"
}

// Check returns ErrBlocked when desktop side effects are disabled, else nil.
func Check() error {
	if Blocked() {
		return ErrBlocked
	}
	return nil
}

// CheckBinary guards launching an external tool that is not a desktop action
// itself (yt-dlp): under go test only binaries inside the temp dir (test
// fakes) may run, so a real tool found on PATH is never executed.
func CheckBinary(path string) error {
	if !testing.Testing() {
		return nil
	}
	tmp, err := filepath.EvalSymlinks(os.TempDir())
	if err != nil {
		tmp = os.TempDir()
	}
	real, err := filepath.EvalSymlinks(path)
	if err != nil {
		real = path
	}
	if rel, err := filepath.Rel(tmp, real); err == nil && !strings.HasPrefix(rel, "..") && rel != "." {
		return nil
	}
	return ErrBlocked
}
