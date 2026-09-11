package config

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// start_on_launch is a legacy duplicate of auto_resume_on_launch that no code
// ever read. It must stay in the file (so nothing that reads the old key breaks)
// but always mirror the field that actually drives behaviour.
func TestStartOnLaunchMirrorsAutoResumeOnLaunch(t *testing.T) {
	for _, want := range []bool{true, false} {
		var s Setting
		s.setDefaults()
		s.AutoResumeOnLaunch = want
		s.StartOnLaunch = !want // deliberately out of step
		s.Validate()

		if s.StartOnLaunch != want {
			t.Fatalf("StartOnLaunch = %v, want it mirrored from AutoResumeOnLaunch (%v)", s.StartOnLaunch, want)
		}
	}
}

// A settings.json where the two disagree resolves to the canonical field, and
// the deprecated key is still written back rather than dropped.
func TestLegacyStartOnLaunchKeyIsKeptButFollowsTheCanonicalField(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", dir)
	path := filepath.Join(dir, "rum", "settings.json")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	body := `{"start_on_launch":true,"auto_resume_on_launch":false,"out_dir":"/tmp"}`
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}

	var s Setting
	if err := s.LoadSettingMetadata(); err != nil {
		t.Fatalf("load: %v", err)
	}
	if s.AutoResumeOnLaunch {
		t.Fatal("the user's explicit auto_resume_on_launch=false was overwritten")
	}
	if s.StartOnLaunch {
		t.Fatal("the deprecated alias did not follow auto_resume_on_launch")
	}

	if err := s.Save(); err != nil {
		t.Fatalf("save: %v", err)
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read back: %v", err)
	}
	var obj map[string]json.RawMessage
	if err := json.Unmarshal(raw, &obj); err != nil {
		t.Fatalf("parse: %v", err)
	}
	if _, ok := obj["start_on_launch"]; !ok {
		t.Fatal("the deprecated key was dropped from settings.json")
	}
}

// A PATCH that still sends the old key is redirected to the canonical field, so
// an older client keeps working instead of writing to a field nothing reads.
func TestUpdateRedirectsStartOnLaunchToAutoResume(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())

	var s Setting
	s.setDefaults()
	s.AutoResumeOnLaunch = true

	off := false
	if err := s.Update(SettingReq{StartOnLaunch: &off}); err != nil {
		t.Fatalf("update: %v", err)
	}
	if s.AutoResumeOnLaunch {
		t.Fatal("start_on_launch=false did not turn auto_resume_on_launch off")
	}
	if s.StartOnLaunch {
		t.Fatal("alias out of step after Update")
	}
}

// When a body carries both, the canonical field wins.
func TestUpdateCanonicalFieldWinsOverTheAlias(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())

	var s Setting
	s.setDefaults()

	on, off := true, false
	if err := s.Update(SettingReq{StartOnLaunch: &off, AutoResumeOnLaunch: &on}); err != nil {
		t.Fatalf("update: %v", err)
	}
	if !s.AutoResumeOnLaunch || !s.StartOnLaunch {
		t.Fatalf("auto_resume_on_launch=%v start_on_launch=%v, want both true",
			s.AutoResumeOnLaunch, s.StartOnLaunch)
	}
}

func TestLogLevelRoundTripsThroughUpdate(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())

	var s Setting
	s.setDefaults()
	debug := "debug"
	if err := s.Update(SettingReq{LogLevel: &debug}); err != nil {
		t.Fatalf("update: %v", err)
	}
	if s.LogLevel != "debug" {
		t.Fatalf("LogLevel = %q, want debug", s.LogLevel)
	}

	var reloaded Setting
	if err := reloaded.LoadSettingMetadata(); err != nil {
		t.Fatalf("load: %v", err)
	}
	if reloaded.LogLevel != "debug" {
		t.Fatalf("reloaded LogLevel = %q, want debug", reloaded.LogLevel)
	}
}

func TestValidLogLevel(t *testing.T) {
	for _, ok := range []string{"debug", "info", "warn", "error"} {
		if !ValidLogLevel(ok) {
			t.Errorf("ValidLogLevel(%q) = false, want true", ok)
		}
	}
	for _, bad := range []string{"", "verbose", "trace", "DEBUG"} {
		if ValidLogLevel(bad) {
			t.Errorf("ValidLogLevel(%q) = true, want false", bad)
		}
	}
}

// The whole point of the alias is that the engine stops reading the dead field.
// This walks the repository and fails if anything outside this package's own
// definition/tests refers to StartOnLaunch or start_on_launch.
func TestNothingOutsideConfigReadsTheDeprecatedField(t *testing.T) {
	root := repoRoot(t)

	var offenders []string
	err := filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return nil // unreadable entry: not this test's business
		}
		if d.IsDir() {
			switch d.Name() {
			case "node_modules", ".git", "dist", "build":
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return nil
		}
		// The canonical definition + alias plumbing lives here by design.
		if filepath.Base(path) == "setting.go" && strings.Contains(path, filepath.Join("pkg", "config")) {
			return nil
		}
		body, readErr := os.ReadFile(path)
		if readErr != nil {
			return nil
		}
		if strings.Contains(string(body), "StartOnLaunch") || strings.Contains(string(body), "start_on_launch") {
			rel, _ := filepath.Rel(root, path)
			offenders = append(offenders, rel)
		}
		return nil
	})
	if err != nil {
		t.Fatalf("walk: %v", err)
	}
	if len(offenders) > 0 {
		t.Fatalf("the deprecated start_on_launch field is still read by: %v", offenders)
	}
}

// repoRoot walks up from the working directory to the checkout root (marked by
// wails.json). The test is skipped when it cannot be located, so it never fails
// spuriously outside a full checkout.
func repoRoot(t *testing.T) string {
	t.Helper()
	dir, err := os.Getwd()
	if err != nil {
		t.Skip("cannot determine working directory")
	}
	for i := 0; i < 8; i++ {
		if _, statErr := os.Stat(filepath.Join(dir, "wails.json")); statErr == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}
	t.Skip("repository root (wails.json) not found; skipping source scan")
	return ""
}
