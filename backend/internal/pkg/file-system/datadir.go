package filesystem

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
)

// EnvDataDir overrides where Rum keeps its data (jobs.json, settings.json,
// extension_token, logs, backups). When unset or empty the default is
// <os.UserConfigDir()>/rum.
const EnvDataDir = "RUM_CONFIG_DIR"

// realUserDataDir is the default data dir as seen when the process started,
// before any test could redirect HOME/XDG. It is what tests must never touch.
var realUserDataDir = defaultDataDir()

func defaultDataDir() string {
	configDir, err := os.UserConfigDir()
	if err != nil || configDir == "" {
		configDir = ".rum"
	}
	return filepath.Join(configDir, "rum")
}

// DataDir returns the directory holding all of Rum's persistent data. It does
// not create it. Under `go test` it panics if the result is the real user data
// dir, so a test can never read or write user data.
func DataDir() string {
	dir := os.Getenv(EnvDataDir)
	if dir == "" {
		dir = defaultDataDir()
	}
	if testing.Testing() && filepath.Clean(dir) == filepath.Clean(realUserDataDir) {
		panic(fmt.Sprintf("filesystem: refusing to use the real user data dir %q under go test; "+
			"set %s (and HOME/XDG_CONFIG_HOME) to a temp dir in TestMain", dir, EnvDataDir))
	}
	return dir
}
