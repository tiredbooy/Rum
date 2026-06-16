package download

import (
	"crypto/md5"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"hash"
	"io"
	"os"
	"strings"
)

// newHasher returns a hash.Hash for the named algorithm. algo is matched
// case-insensitively against the supported set {"sha256", "md5"}. The ok return
// is false for an unsupported/unknown algorithm.
func newHasher(algo string) (hash.Hash, bool) {
	switch strings.ToLower(strings.TrimSpace(algo)) {
	case "sha256":
		return sha256.New(), true
	case "md5":
		return md5.New(), true
	default:
		return nil, false
	}
}

// VerifyChecksum streams the file at path through the hasher for algo and
// compares the lowercase hex digest against expected (also normalized to
// lowercase hex, tolerating surrounding whitespace). It is meant to run after a
// download is fully assembled, to catch silent corruption / truncation.
//
// Contract:
//   - expected == "" (after trimming) is a no-op and returns nil. This lets the
//     caller wire VerifyChecksum unconditionally even when no checksum is known.
//   - algo must be one of {"sha256","md5"} (case-insensitive); otherwise an
//     error is returned (not wrapped in ErrChecksumMismatch — it is a usage/
//     configuration error, not a content mismatch).
//   - On digest mismatch the returned error wraps ErrChecksumMismatch.
//   - The file is streamed via io.Copy into the hasher; it is never fully read
//     into memory.
func VerifyChecksum(path, algo, expected string) error {
	expected = strings.TrimSpace(expected)
	if expected == "" {
		// No checksum to verify against: explicit no-op.
		return nil
	}

	hasher, ok := newHasher(algo)
	if !ok {
		return fmt.Errorf("verify checksum: unsupported algorithm %q", algo)
	}

	f, err := os.Open(path)
	if err != nil {
		return fmt.Errorf("verify checksum: open %q: %w", path, err)
	}
	defer f.Close()

	if _, err := io.Copy(hasher, f); err != nil {
		return fmt.Errorf("verify checksum: read %q: %w", path, err)
	}

	got := hex.EncodeToString(hasher.Sum(nil))
	want := strings.ToLower(expected)
	if got != want {
		return fmt.Errorf("verify checksum: %s mismatch for %q: got %s want %s: %w",
			strings.ToLower(algo), path, got, want, ErrChecksumMismatch)
	}
	return nil
}
