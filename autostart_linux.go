//go:build linux

package main

import (
	"fmt"
	"os"
	"path/filepath"
)

// autostartFileName is the .desktop entry written into the XDG autostart dir.
const autostartFileName = "rum.desktop"

// autostartDir returns ~/.config/autostart, honoring XDG_CONFIG_HOME.
func autostartDir() (string, error) {
	cfg, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(cfg, "autostart"), nil
}

// SetAutostart creates or removes the XDG autostart entry so Rum launches on
// login. enable=true writes ~/.config/autostart/rum.desktop pointing at the
// current executable; enable=false removes it (a missing file is not an error).
func SetAutostart(enable bool) error {
	dir, err := autostartDir()
	if err != nil {
		return err
	}
	path := filepath.Join(dir, autostartFileName)

	if !enable {
		if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
			return err
		}
		return nil
	}

	exe, err := os.Executable()
	if err != nil {
		return err
	}
	exe, _ = filepath.Abs(exe)

	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}

	content := fmt.Sprintf(`[Desktop Entry]
Type=Application
Name=Rum
Comment=Powerful, fast and modern download manager
Exec=%s
Icon=rum
Terminal=false
Categories=Network;FileTransfer;
StartupWMClass=Rum
X-GNOME-Autostart-enabled=true
`, exe)

	return os.WriteFile(path, []byte(content), 0o644)
}
