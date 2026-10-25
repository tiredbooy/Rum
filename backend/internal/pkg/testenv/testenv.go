// Package testenv points every per-user path Rum resolves (HOME, XDG, APPDATA,
// RUM_CONFIG_DIR) at a throwaway directory so a test binary can never read or
// write the developer's real data. Call it from TestMain.
package testenv

import (
	"os"
	"path/filepath"
)

// Run isolates the environment, runs the tests via m.Run and removes the temp
// dir. Use: func TestMain(m *testing.M) { os.Exit(testenv.Run(m)) }
func Run(m interface{ Run() int }) int {
	tmp, err := os.MkdirTemp("", "rum-test-*")
	if err != nil {
		panic(err)
	}
	defer os.RemoveAll(tmp)
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
	return m.Run()
}
