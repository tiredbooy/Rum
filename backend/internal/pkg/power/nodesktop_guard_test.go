package power

import (
	"errors"
	"testing"

	"github.com/tiredbooy/Rum/backend/nodesktop"
)

func TestSystemBackendBlockedUnderTest(t *testing.T) {
	if _, err := SystemBackend().Acquire("test"); !errors.Is(err, nodesktop.ErrBlocked) {
		t.Fatalf("Acquire = %v, want ErrBlocked", err)
	}
}
