//go:build windows

package main

import (
	"os"

	"golang.org/x/sys/windows/registry"
)

// runKeyPath is the per-user (no admin needed) Run key Windows reads at login.
const runKeyPath = `Software\Microsoft\Windows\CurrentVersion\Run`

// runValueName is the value under the Run key that points at the Rum exe.
const runValueName = "Rum"

// SetAutostart adds or removes the HKCU Run-key entry so Rum launches on login.
// enable=true sets HKCU\...\Run\Rum to the current executable path; enable=false
// deletes the value (a missing value is not an error).
func SetAutostart(enable bool) error {
	key, err := registry.OpenKey(registry.CURRENT_USER, runKeyPath, registry.SET_VALUE)
	if err != nil {
		return err
	}
	defer key.Close()

	if !enable {
		if err := key.DeleteValue(runValueName); err != nil && err != registry.ErrNotExist {
			return err
		}
		return nil
	}

	exe, err := os.Executable()
	if err != nil {
		return err
	}
	// Quote the path so a space in the install location doesn't split the command.
	return key.SetStringValue(runValueName, `"`+exe+`"`)
}
