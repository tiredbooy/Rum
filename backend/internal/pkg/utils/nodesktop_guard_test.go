package utils

import (
	"errors"
	"testing"

	"github.com/tiredbooy/Rum/backend/nodesktop"
)

// Power actions must never run for real under go test.
func TestPowerActionsBlockedUnderTest(t *testing.T) {
	if err := ShutdownPC(); !errors.Is(err, nodesktop.ErrBlocked) {
		t.Fatalf("ShutdownPC = %v, want ErrBlocked", err)
	}
	if err := SleepPC(); !errors.Is(err, nodesktop.ErrBlocked) {
		t.Fatalf("SleepPC = %v, want ErrBlocked", err)
	}
}
