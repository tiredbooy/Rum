package filesystem

import (
	"errors"
	"testing"

	"github.com/tiredbooy/Rum/backend/nodesktop"
)

func TestOpenFolderBlockedUnderTest(t *testing.T) {
	if err := OpenFolder(t.TempDir()); !errors.Is(err, nodesktop.ErrBlocked) {
		t.Fatalf("OpenFolder = %v, want ErrBlocked", err)
	}
}
