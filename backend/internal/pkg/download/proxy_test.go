package download

import (
	"net/http"
	"testing"

	"github.com/tiredbooy/Rum/backend/internal/pkg/config"
)

// TestProxyAppliedToTransport guards B4: the persisted Proxy setting was shown
// in the UI but never wired into any HTTP transport. NewDownloader must now apply
// it, including socks5.
func TestProxyAppliedToTransport(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	var s config.Setting
	_ = s.LoadSettingMetadata()
	s.Proxy = "socks5://127.0.0.1:1080"
	if err := s.Save(); err != nil {
		t.Fatalf("save settings: %v", err)
	}

	d := NewDownloader("ua", "")
	tr, ok := d.Client.Transport.(*http.Transport)
	if !ok {
		t.Fatalf("transport is not *http.Transport, got %T", d.Client.Transport)
	}
	if tr.Proxy == nil {
		t.Fatal("Transport.Proxy is nil; the proxy setting was not applied")
	}
	req, _ := http.NewRequest("GET", "https://example.com/file", nil)
	u, err := tr.Proxy(req)
	if err != nil {
		t.Fatalf("proxy func returned error: %v", err)
	}
	if u == nil || u.String() != "socks5://127.0.0.1:1080" {
		t.Fatalf("proxy url = %v, want socks5://127.0.0.1:1080", u)
	}
}

// TestProxyBareHostDefaultsToHTTP checks a "host:port" value (no scheme) is
// treated as an http proxy rather than ignored.
func TestProxyBareHostDefaultsToHTTP(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	var s config.Setting
	_ = s.LoadSettingMetadata()
	s.Proxy = "127.0.0.1:8080"
	if err := s.Save(); err != nil {
		t.Fatalf("save settings: %v", err)
	}
	d := NewDownloader("ua", "")
	tr := d.Client.Transport.(*http.Transport)
	if tr.Proxy == nil {
		t.Fatal("bare host:port proxy was not applied")
	}
	req, _ := http.NewRequest("GET", "https://example.com", nil)
	u, _ := tr.Proxy(req)
	if u == nil || u.String() != "http://127.0.0.1:8080" {
		t.Fatalf("proxy url = %v, want http://127.0.0.1:8080", u)
	}
}

// TestProxyEmptyMeansNoProxy ensures the default (no proxy) path is a true no-op.
func TestProxyEmptyMeansNoProxy(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	var s config.Setting
	_ = s.LoadSettingMetadata()
	s.Proxy = ""
	_ = s.Save()
	d := NewDownloader("ua", "")
	tr := d.Client.Transport.(*http.Transport)
	if tr.Proxy != nil {
		req, _ := http.NewRequest("GET", "https://example.com", nil)
		if u, _ := tr.Proxy(req); u != nil {
			t.Fatalf("expected no proxy, got %v", u)
		}
	}
}

func TestIsSupportedProxyScheme(t *testing.T) {
	for _, s := range []string{"http", "https", "socks5", "socks5h", "SOCKS5"} {
		if !isSupportedProxyScheme(s) {
			t.Errorf("scheme %q should be supported", s)
		}
	}
	for _, s := range []string{"ftp", "ssh", "", "socks4", "file"} {
		if isSupportedProxyScheme(s) {
			t.Errorf("scheme %q should NOT be supported", s)
		}
	}
}
