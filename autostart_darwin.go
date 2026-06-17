//go:build darwin

package main

import (
	"fmt"
	"os"
	"path/filepath"
)

// launchAgentLabel is the LaunchAgent label / plist filename for Rum.
const launchAgentLabel = "ir.tiredbooy.rum"

// launchAgentPath returns ~/Library/LaunchAgents/ir.tiredbooy.rum.plist.
func launchAgentPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, "Library", "LaunchAgents", launchAgentLabel+".plist"), nil
}

// SetAutostart writes or removes a per-user LaunchAgent so Rum launches at login.
// enable=true writes ~/Library/LaunchAgents/ir.tiredbooy.rum.plist with RunAtLoad
// pointing at the current executable; enable=false removes it (missing is not an
// error).
//
// NOTE: this is best-effort and untested (no macOS in the build environment). The
// LaunchAgent only takes effect after the user logs in again or runs `launchctl
// load` on the plist.
func SetAutostart(enable bool) error {
	path, err := launchAgentPath()
	if err != nil {
		return err
	}

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

	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}

	content := fmt.Sprintf(`<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
	<key>Label</key>
	<string>%s</string>
	<key>ProgramArguments</key>
	<array>
		<string>%s</string>
	</array>
	<key>RunAtLoad</key>
	<true/>
</dict>
</plist>
`, launchAgentLabel, exe)

	return os.WriteFile(path, []byte(content), 0o644)
}
