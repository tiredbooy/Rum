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
