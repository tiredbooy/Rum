package updater

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func sum(b []byte) string { h := sha256.Sum256(b); return hex.EncodeToString(h[:]) }

func TestParseSums(t *testing.T) {
	body := []byte("# comment\n" +
		sum([]byte("a")) + "  Rum-linux.AppImage\n" +
		sum([]byte("b")) + " *Rum-setup.exe\r\n" +
		"nothex  bad.bin\n\n")
	m := ParseSums(body)
	if len(m) != 2 || m["Rum-linux.AppImage"] != sum([]byte("a")) || m["Rum-setup.exe"] != sum([]byte("b")) {
		t.Errorf("parsed = %v", m)
	}
}

func TestVerifyFile(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "x.bin")
	if err := os.WriteFile(p, []byte("payload"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := VerifyFile(p, sum([]byte("payload"))); err != nil {
		t.Errorf("good sum: %v", err)
	}
	if err := VerifyFile(p, "  "+sum([]byte("payload"))[:10]+"  "); !errors.Is(err, ErrChecksumMismatch) {
		t.Errorf("truncated sum: %v", err)
	}
	if err := VerifyFile(p, sum([]byte("other"))); !errors.Is(err, ErrChecksumMismatch) {
		t.Errorf("bad sum: %v", err)
	}
	if err := VerifyFile(filepath.Join(dir, "missing"), sum(nil)); err == nil {
		t.Error("missing file must error")
	}
}
