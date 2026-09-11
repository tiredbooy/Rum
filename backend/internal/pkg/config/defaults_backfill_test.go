package config

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

// writeSettingsFile drops a raw settings.json into a temp config dir and points
// the process at it, mirroring what filesystem.ReadMetadataFile reads.
func writeSettingsFile(t *testing.T, body string) {
	t.Helper()
	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", dir)
	path := filepath.Join(dir, "rum", "settings.json")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatalf("write settings: %v", err)
	}
}

// A settings.json written before the reliability fields existed must come back
// with those fields ON. Previously a missing key unmarshalled to false and was
// indistinguishable from a deliberate opt-out, so every upgrader silently ran
// with integrity verification and auto-resume disabled while the UI drew them
// as enabled.
func TestMissingReliabilityKeysDefaultToOn(t *testing.T) {
	writeSettingsFile(t, `{"out_dir":"/tmp","max_parallel":2,"connections":4}`)

	var s Setting
	if err := s.LoadSettingMetadata(); err != nil {
		t.Fatalf("load: %v", err)
	}

	if !s.VerifyIntegrity {
		t.Error("VerifyIntegrity should default to true when the key is absent")
	}
	if !s.AutoResumeOnReconnect {
		t.Error("AutoResumeOnReconnect should default to true when the key is absent")
	}
	if !s.AutoResumeOnLaunch {
		t.Error("AutoResumeOnLaunch should default to true when the key is absent")
	}
	if !s.KeepPartialOnFailure {
		t.Error("KeepPartialOnFailure should default to true when the key is absent")
	}
}

// The mirror image: a key that IS present as false is the user's choice and must
// survive the backfill.
func TestExplicitFalseReliabilityKeysArePreserved(t *testing.T) {
	writeSettingsFile(t, `{
		"out_dir":"/tmp",
		"verify_integrity":false,
		"auto_resume_on_reconnect":false,
		"auto_resume_on_launch":false,
		"keep_partial_on_failure":false
	}`)

	var s Setting
	if err := s.LoadSettingMetadata(); err != nil {
		t.Fatalf("load: %v", err)
	}

	if s.VerifyIntegrity || s.AutoResumeOnReconnect || s.AutoResumeOnLaunch || s.KeepPartialOnFailure {
		t.Fatalf("explicit false was overwritten by defaults: %+v", s)
	}
}

func TestPresentKeysIgnoresNonObjectBodies(t *testing.T) {
	if got := presentKeys([]byte(`["not","an","object"]`)); got != nil {
		t.Fatalf("presentKeys(array) = %v, want nil", got)
	}
	got := presentKeys([]byte(`{"a":1,"b":false}`))
	if !got["a"] || !got["b"] || got["c"] {
		t.Fatalf("presentKeys mismatch: %v", got)
	}
}

func TestNormalizeProxy(t *testing.T) {
	cases := []struct {
		in     string
		want   string
		wantOK bool
	}{
		{"", "", true},
		{"  ", "", true},
		{"127.0.0.1:1080", "http://127.0.0.1:1080", true},
		{"http://user:pass@host:3128", "http://user:pass@host:3128", true},
		{"socks5://127.0.0.1:9050", "socks5://127.0.0.1:9050", true},
		{"socks5h://127.0.0.1:9050", "socks5h://127.0.0.1:9050", true},
		{"ftp://host:21", "", false},
		{"http://", "", false},
		{"not a url at all", "", false},
	}
	for _, tc := range cases {
		got, ok := NormalizeProxy(tc.in)
		if ok != tc.wantOK || (ok && got != tc.want) {
			t.Errorf("NormalizeProxy(%q) = (%q, %v), want (%q, %v)", tc.in, got, ok, tc.want, tc.wantOK)
		}
	}
}

// An unusable proxy must not survive a save: the engine would ignore it, and
// leaving it in the file made the settings UI show a proxy that was not in force.
func TestValidateDropsUnusableProxy(t *testing.T) {
	s := Setting{Proxy: "ftp://nope:21"}
	s.Validate()
	if s.Proxy != "" {
		t.Fatalf("Proxy = %q, want empty after Validate", s.Proxy)
	}

	s = Setting{Proxy: "127.0.0.1:8888"}
	s.Validate()
	if s.Proxy != "http://127.0.0.1:8888" {
		t.Fatalf("Proxy = %q, want normalized http URL", s.Proxy)
	}
}

func TestValidateRejectsUnknownTheme(t *testing.T) {
	s := Setting{PreferredTheme: "midnight"}
	s.Validate()
	if s.PreferredTheme != "system" {
		t.Fatalf("PreferredTheme = %q, want system", s.PreferredTheme)
	}
}

// A round trip through Save/Load must not resurrect a disabled default-true flag.
func TestSaveLoadRoundTripKeepsDisabledFlags(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", dir)

	var s Setting
	s.setDefaults()
	s.VerifyIntegrity = false
	if err := s.Save(); err != nil {
		t.Fatalf("save: %v", err)
	}

	// Sanity: the key really is in the file (that is what makes the round trip work).
	raw, err := os.ReadFile(filepath.Join(dir, "rum", "settings.json"))
	if err != nil {
		t.Fatalf("read back: %v", err)
	}
	var obj map[string]json.RawMessage
	if err := json.Unmarshal(raw, &obj); err != nil {
		t.Fatalf("parse: %v", err)
	}
	if _, ok := obj["verify_integrity"]; !ok {
		t.Fatal("verify_integrity missing from saved file")
	}

	var loaded Setting
	if err := loaded.LoadSettingMetadata(); err != nil {
		t.Fatalf("load: %v", err)
	}
	if loaded.VerifyIntegrity {
		t.Fatal("VerifyIntegrity came back true after being saved as false")
	}
}
