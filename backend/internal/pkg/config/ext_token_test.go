package config

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestExtensionTokenCreatedOnceAndStable(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())

	a, err := ExtensionToken()
	if err != nil {
		t.Fatalf("ExtensionToken: %v", err)
	}
	if !strings.HasPrefix(a, "rum_") || len(a) < 4+64 {
		t.Fatalf("token %q is not a long random rum_ token", a)
	}
	b, _ := ExtensionToken()
	if a != b {
		t.Fatal("token changed between reads")
	}
}

func TestExtensionTokenFileIsPrivate(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", dir)
	if _, err := ExtensionToken(); err != nil {
		t.Fatal(err)
	}
	st, err := os.Stat(filepath.Join(dir, "rum", extTokenFile))
	if err != nil {
		t.Fatal(err)
	}
	if runtime.GOOS != "windows" && st.Mode().Perm() != 0o600 {
		t.Fatalf("token file perms = %o, want 600", st.Mode().Perm())
	}
}

func TestRegenerateExtensionTokenInvalidatesOld(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	old, _ := ExtensionToken()
	fresh, err := RegenerateExtensionToken()
	if err != nil {
		t.Fatal(err)
	}
	if fresh == old {
		t.Fatal("regenerate returned the same token")
	}
	if VerifyExtensionToken(old) {
		t.Fatal("old token still accepted")
	}
	if !VerifyExtensionToken(fresh) {
		t.Fatal("new token rejected")
	}
}

func TestVerifyExtensionToken(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	tok, _ := ExtensionToken()
	cases := map[string]bool{
		tok:                  true,
		"":                   false,
		"wrong":              false,
		tok + "x":            false,
		tok[:len(tok)-1]:     false,
		" " + tok:            false,
		strings.ToUpper(tok): false,
	}
	for in, want := range cases {
		if got := VerifyExtensionToken(in); got != want {
			t.Errorf("Verify(%q) = %v, want %v", in, got, want)
		}
	}
}

// A truncated/garbage token file must not become a weak token.
func TestExtensionTokenReplacesWeakFile(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", dir)
	p := filepath.Join(dir, "rum", extTokenFile)
	_ = os.MkdirAll(filepath.Dir(p), 0o755)
	_ = os.WriteFile(p, []byte("abc"), 0o600)
	tok, err := ExtensionToken()
	if err != nil || tok == "abc" || len(tok) < 32 {
		t.Fatalf("weak token kept: %q %v", tok, err)
	}
}
