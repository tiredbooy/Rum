package download

import "testing"

func TestShouldFreeOSMemory(t *testing.T) {
	if !shouldFreeOSMemory(0) {
		t.Fatal("idle (0 active) should free OS memory")
	}
	if shouldFreeOSMemory(1) {
		t.Fatal("with active downloads it should not force-free")
	}
}
