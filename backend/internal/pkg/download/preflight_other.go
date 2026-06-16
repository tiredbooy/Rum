//go:build !unix && !windows

package download

// availableDiskBytes is the portable fallback for platforms where we have no
// supported way to query free space (e.g. js/wasm, plan9). It always reports
// "undeterminable" so PreflightDiskSpace degrades to a no-op rather than
// blocking downloads it cannot prove will fail.
func availableDiskBytes(dir string) (int64, bool) {
	return 0, false
}
