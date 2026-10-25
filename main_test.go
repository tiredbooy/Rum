package main

import (
	"os"
	"path/filepath"
	"testing"
)

// TestMain keeps every test off the real user config dir: HOME, XDG, APPDATA
// and RUM_CONFIG_DIR all point at a throwaway dir removed afterwards.
func TestMain(m *testing.M) {
	tmp, err := os.MkdirTemp("", "rum-test-*")
	if err != nil {
		panic(err)
	}
	for k, v := range map[string]string{
		"HOME":            tmp,
		"USERPROFILE":     tmp,
		"XDG_CONFIG_HOME": filepath.Join(tmp, ".config"),
		"XDG_DATA_HOME":   filepath.Join(tmp, ".local", "share"),
		"APPDATA":         filepath.Join(tmp, "AppData", "Roaming"),
		"LOCALAPPDATA":    filepath.Join(tmp, "AppData", "Local"),
		"RUM_CONFIG_DIR":  filepath.Join(tmp, "rum"),
	} {
		os.Setenv(k, v)
	}
	code := m.Run()
	os.RemoveAll(tmp)
	os.Exit(code)
}
