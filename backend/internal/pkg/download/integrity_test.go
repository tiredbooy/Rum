package download

import (
	"crypto/md5"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// writeTempFile writes content to a fresh file under t.TempDir and returns its
// path.
func writeTempFile(t *testing.T, content []byte) string {
	t.Helper()
	dir := t.TempDir()
	p := filepath.Join(dir, "data.bin")
	if err := os.WriteFile(p, content, 0644); err != nil {
		t.Fatalf("write temp file: %v", err)
	}
	return p
}

func sha256Hex(b []byte) string {
	s := sha256.Sum256(b)
	return hex.EncodeToString(s[:])
}

func md5Hex(b []byte) string {
	s := md5.Sum(b)
	return hex.EncodeToString(s[:])
}

func TestVerifyChecksum(t *testing.T) {
	payload := []byte("the quick brown fox jumps over the lazy dog")
	good256 := sha256Hex(payload)
	goodMD5 := md5Hex(payload)

	tests := []struct {
		name     string
		algo     string
		expected string
		wantErr  error // sentinel to match with errors.Is, or nil for no error
		wantAny  bool  // expect some non-nil error not tied to a sentinel
	}{
		{name: "sha256 match", algo: "sha256", expected: good256, wantErr: nil},
		{name: "sha256 match uppercase algo", algo: "SHA256", expected: good256, wantErr: nil},
		{name: "sha256 match uppercase expected", algo: "sha256", expected: strings.ToUpper(good256), wantErr: nil},
		{name: "sha256 match with whitespace", algo: "sha256", expected: "  " + good256 + "\n", wantErr: nil},
		{name: "md5 match", algo: "md5", expected: goodMD5, wantErr: nil},
		{name: "md5 match mixed case algo", algo: "Md5", expected: goodMD5, wantErr: nil},
		{name: "sha256 mismatch", algo: "sha256", expected: strings.Repeat("0", 64), wantErr: ErrChecksumMismatch},
		{name: "md5 mismatch", algo: "md5", expected: strings.Repeat("a", 32), wantErr: ErrChecksumMismatch},
		{name: "empty expected is noop", algo: "sha256", expected: "", wantErr: nil},
		{name: "whitespace-only expected is noop", algo: "sha256", expected: "   ", wantErr: nil},
		{name: "empty expected unsupported algo still noop", algo: "crc32", expected: "", wantErr: nil},
		{name: "unsupported algo", algo: "crc32", expected: good256, wantAny: true},
		{name: "empty algo with expected", algo: "", expected: good256, wantAny: true},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			path := writeTempFile(t, payload)
			err := VerifyChecksum(path, tc.algo, tc.expected)

			switch {
			case tc.wantAny:
				if err == nil {
					t.Fatalf("expected an error, got nil")
				}
				if errors.Is(err, ErrChecksumMismatch) {
					t.Fatalf("unsupported-algo error must not be a checksum mismatch: %v", err)
				}
			case tc.wantErr != nil:
				if !errors.Is(err, tc.wantErr) {
					t.Fatalf("expected error wrapping %v, got %v", tc.wantErr, err)
				}
			default:
				if err != nil {
					t.Fatalf("expected nil error, got %v", err)
				}
			}
		})
	}
}

func TestVerifyChecksum_MissingFile(t *testing.T) {
	// A missing file is an open error, not a content mismatch.
	err := VerifyChecksum(filepath.Join(t.TempDir(), "nope.bin"), "sha256", strings.Repeat("0", 64))
	if err == nil {
		t.Fatal("expected error for missing file")
	}
	if errors.Is(err, ErrChecksumMismatch) {
		t.Fatalf("missing file should not classify as checksum mismatch: %v", err)
	}
}

func TestVerifyChecksum_EmptyFile(t *testing.T) {
	path := writeTempFile(t, []byte{})
	// sha256 of empty input.
	empty := sha256Hex([]byte{})
	if err := VerifyChecksum(path, "sha256", empty); err != nil {
		t.Fatalf("empty file checksum should match: %v", err)
	}
}

func TestNewHasher(t *testing.T) {
	tests := []struct {
		algo string
		ok   bool
	}{
		{"sha256", true},
		{"SHA256", true},
		{" sha256 ", true},
		{"md5", true},
		{"MD5", true},
		{"sha1", false},
		{"", false},
	}
	for _, tc := range tests {
		_, ok := newHasher(tc.algo)
		if ok != tc.ok {
			t.Errorf("newHasher(%q) ok = %v, want %v", tc.algo, ok, tc.ok)
		}
	}
}
