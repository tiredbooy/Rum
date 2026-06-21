package download

import (
	"context"
	"net"
	"strings"
	"testing"
)

// TestSSRFGuardRejectsInternalHosts guards B5: when enabled, the dial guard must
// refuse loopback, link-local (cloud metadata), and private targets.
func TestSSRFGuardRejectsInternalHosts(t *testing.T) {
	dial := ssrfGuardedDialContext(&net.Dialer{})
	for _, addr := range []string{
		"127.0.0.1:80",
		"169.254.169.254:80", // cloud metadata endpoint (link-local)
		"10.0.0.1:80",
		"192.168.1.10:80",
		"[::1]:80",
	} {
		_, err := dial(context.Background(), "tcp", addr)
		if err == nil {
			t.Errorf("SSRF guard should reject %s", addr)
			continue
		}
		if !strings.Contains(err.Error(), "private") && !strings.Contains(err.Error(), "internal") {
			t.Errorf("addr %s: error %q should explain it is private/internal", addr, err)
		}
	}
}
