package updater

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
)

// ErrChecksumMismatch means the file does not match its published SHA-256.
var ErrChecksumMismatch = errors.New("checksum mismatch")

// ParseSums reads a SHA256SUMS file into name -> lowercase hex digest. Lines that
// are not "<64 hex>  <name>" (comments, blanks, junk) are skipped.
func ParseSums(body []byte) map[string]string {
	out := map[string]string{}
	for _, line := range strings.Split(string(body), "\n") {
		fields := strings.Fields(line)
		if len(fields) != 2 || len(fields[0]) != 64 {
			continue
		}
		if _, err := hex.DecodeString(fields[0]); err != nil {
			continue
		}
		out[strings.TrimPrefix(fields[1], "*")] = strings.ToLower(fields[0])
	}
	return out
}

// VerifyFile hashes the file at path and compares it with want (hex SHA-256).
func VerifyFile(path, want string) error {
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer f.Close()
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return err
	}
	got := hex.EncodeToString(h.Sum(nil))
	if got != strings.ToLower(strings.TrimSpace(want)) {
		return fmt.Errorf("%w: got %s", ErrChecksumMismatch, got)
	}
	return nil
}
