package power

import (
	"errors"

	"github.com/tiredbooy/Rum/backend/nodesktop"
)

// ErrUnsupported means this platform has no way to inhibit sleep.
var ErrUnsupported = errors.New("sleep inhibit not supported")

// SystemBackend returns the OS-specific backend (see inhibit_*.go).
func SystemBackend() Backend { return guardedBackend{systemBackend()} }

// guardedBackend refuses real inhibition under test or RUM_NO_DESKTOP=1.
type guardedBackend struct{ Backend }

func (g guardedBackend) Acquire(reason string) (func(), error) {
	if err := nodesktop.Check(); err != nil {
		return nil, err
	}
	return g.Backend.Acquire(reason)
}
