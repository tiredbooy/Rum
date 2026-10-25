package power

import "errors"

// ErrUnsupported means this platform has no way to inhibit sleep.
var ErrUnsupported = errors.New("sleep inhibit not supported")

// SystemBackend returns the OS-specific backend (see inhibit_*.go).
func SystemBackend() Backend { return systemBackend() }
