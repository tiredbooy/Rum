//go:build windows

package download

import (
	"golang.org/x/sys/windows"
)

// availableDiskBytes returns the bytes available to the caller on the volume
// holding dir using GetDiskFreeSpaceEx (via golang.org/x/sys/windows). The
// "free bytes available to the caller" value already accounts for per-user
// quotas, mirroring the Unix Bavail semantics.
func availableDiskBytes(dir string) (int64, bool) {
	if dir == "" {
		return 0, false
	}
	p, err := windows.UTF16PtrFromString(dir)
	if err != nil {
		return 0, false
	}
	var freeAvail, totalBytes, totalFree uint64
	if err := windows.GetDiskFreeSpaceEx(p, &freeAvail, &totalBytes, &totalFree); err != nil {
		return 0, false
	}
	if freeAvail > (1 << 62) {
		// Implausibly large; clamp to avoid int64 overflow surprises.
		return 1<<63 - 1, true
	}
	return int64(freeAvail), true
}
