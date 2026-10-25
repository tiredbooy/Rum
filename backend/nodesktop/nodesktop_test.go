package nodesktop

import (
	"errors"
	"testing"
)

func TestBlockedUnderTest(t *testing.T) {
	if !Blocked() {
		t.Fatal("Blocked must be true inside go test")
	}
	if !errors.Is(Check(), ErrBlocked) {
		t.Fatal("Check must return ErrBlocked inside go test")
	}
}

func TestBlockedByEnv(t *testing.T) {
	t.Setenv("RUM_NO_DESKTOP", "1")
	if !Blocked() {
		t.Fatal("RUM_NO_DESKTOP=1 must block")
	}
}

func TestCheckBinaryOnlyAllowsTempFakes(t *testing.T) {
	if err := CheckBinary("/usr/bin/yt-dlp"); !errors.Is(err, ErrBlocked) {
		t.Fatalf("real binary = %v, want ErrBlocked", err)
	}
	if err := CheckBinary(t.TempDir() + "/yt-dlp"); err != nil {
		t.Fatalf("temp fake = %v, want nil", err)
	}
}
