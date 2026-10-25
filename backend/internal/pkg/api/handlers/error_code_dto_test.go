package handlers

import (
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/tiredbooy/Rum/backend/internal/pkg/download"
)

func TestDownloadResponseCarriesErrorCode(t *testing.T) {
	j := &download.Job{ID: "a", Status: download.StatusError}
	j.SetError(errors.New("dial tcp 1.2.3.4:443: i/o timeout"))
	b, _ := json.Marshal(toDownloadResponseFull(j))
	s := string(b)
	for _, want := range []string{`"error_code":"timeout"`, `"retryable":true`} {
		if !strings.Contains(s, want) {
			t.Fatalf("%s missing %s", s, want)
		}
	}

	ok := &download.Job{ID: "b", Status: download.StatusCompleted}
	b, _ = json.Marshal(toDownloadResponseFull(ok))
	if strings.Contains(string(b), "error_code") || strings.Contains(string(b), "retryable") {
		t.Fatalf("omitempty violated: %s", b)
	}
}
