package main

import (
	"context"
	"errors"
	"testing"

	"github.com/tiredbooy/Rum/backend/nodesktop"
)

// The real exec / notification paths must refuse to run under go test.
func TestDesktopSideEffectsBlockedUnderTest(t *testing.T) {
	if err := execArgv([]string{"xdg-open", t.TempDir()}); !errors.Is(err, nodesktop.ErrBlocked) {
		t.Fatalf("execArgv = %v, want ErrBlocked", err)
	}
	if _, err := systemSend(context.Background(), "t", "b"); !errors.Is(err, nodesktop.ErrBlocked) {
		t.Fatalf("systemSend = %v, want ErrBlocked", err)
	}
	if err := notifyDesktop("t", "b"); !errors.Is(err, nodesktop.ErrBlocked) {
		t.Fatalf("notifyDesktop = %v, want ErrBlocked", err)
	}
}
