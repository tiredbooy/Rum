package download

import (
	"errors"
	"testing"
)

func TestRequiredWithMargin(t *testing.T) {
	tests := []struct {
		name string
		need int64
		want int64
	}{
		{"zero uses flat margin", 0, diskSpaceFlatMargin},
		{"negative uses flat margin", -5, diskSpaceFlatMargin},
		{"small uses flat margin", 1000, 1000 + diskSpaceFlatMargin},
		// 10 GiB: 1% (>16MiB) dominates.
		{"large uses one percent", 10 * 1024 * 1024 * 1024, 10*1024*1024*1024 + (10*1024*1024*1024)/100},
		{"overflow clamps", 1<<63 - 1, 1<<63 - 1},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := requiredWithMargin(tc.need); got != tc.want {
				t.Errorf("requiredWithMargin(%d) = %d, want %d", tc.need, got, tc.want)
			}
		})
	}
}

func TestPreflightDiskSpace_UnknownSize(t *testing.T) {
	// needBytes <= 0 is always a no-op regardless of the directory.
	if err := PreflightDiskSpace(t.TempDir(), 0); err != nil {
		t.Errorf("expected nil for unknown size, got %v", err)
	}
	if err := PreflightDiskSpace(t.TempDir(), -100); err != nil {
		t.Errorf("expected nil for negative size, got %v", err)
	}
}

func TestPreflightDiskSpace_PlausibleFits(t *testing.T) {
	// A tiny request against a real temp dir should comfortably fit (or degrade
	// to nil if free space is undeterminable on this platform). Either way: nil.
	if err := PreflightDiskSpace(t.TempDir(), 1024); err != nil {
		t.Errorf("expected small download to fit, got %v", err)
	}
}

func TestPreflightDiskSpace_HugeRequest(t *testing.T) {
	dir := t.TempDir()
	avail, ok := availableDiskBytes(dir)
	if !ok {
		// Platform cannot determine free space: preflight must degrade to nil.
		if err := PreflightDiskSpace(dir, 1<<62); err != nil {
			t.Fatalf("undeterminable platform must not block: %v", err)
		}
		t.Skip("free space undeterminable on this platform; skipping insufficient-space assertion")
	}
	// Request far more than is available -> must wrap ErrInsufficientDiskSpace.
	huge := avail + (1 << 40) // available + 1 TiB
	err := PreflightDiskSpace(dir, huge)
	if err == nil {
		t.Fatalf("expected insufficient-space error for %d bytes (avail %d)", huge, avail)
	}
	if !errors.Is(err, ErrInsufficientDiskSpace) {
		t.Fatalf("expected error wrapping ErrInsufficientDiskSpace, got %v", err)
	}
}

func TestAvailableDiskBytes_EmptyDir(t *testing.T) {
	// An empty path must report undeterminable, never a bogus positive number.
	if _, ok := availableDiskBytes(""); ok {
		t.Error("expected ok=false for empty dir")
	}
}
