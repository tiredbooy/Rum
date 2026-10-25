// Package nodesktop is the single switch that stops Rum from touching the
// user's desktop (file managers, notifications, power actions, sleep
// inhibition, tray) while tests run or when RUM_NO_DESKTOP=1 is set.
package nodesktop

import (
	"errors"
	"os"
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
