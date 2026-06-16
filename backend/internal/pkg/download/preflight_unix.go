//go:build unix

package download

import "golang.org/x/sys/unix"

// availableDiskBytes returns the number of bytes available to an unprivileged
// process on the filesystem that holds dir, and ok == true when it could be
// determined. On Unix-like systems it uses statfs(2) via golang.org/x/sys/unix.
//
// We deliberately use Bavail (blocks available to non-root) rather than Bfree
// (all free blocks, including the root-reserved pool) so the estimate matches
// what the downloader can actually write.
func availableDiskBytes(dir string) (int64, bool) {
	if dir == "" {
		return 0, false
	}
	var st unix.Statfs_t
	if err := unix.Statfs(dir, &st); err != nil {
		return 0, false
	}
	// Bsize can be reported as a signed or unsigned int depending on the OS;
	// normalize through uint64 then guard for non-positive / overflow.
	bsize := int64(st.Bsize)
	if bsize <= 0 {
		return 0, false
	}
	avail := int64(st.Bavail)
	if avail < 0 {
		return 0, false
	}
	// Overflow guard on avail*bsize.
	if avail != 0 && (1<<63-1)/avail < bsize {
		return 1<<63 - 1, true
	}
	return avail * bsize, true
}
