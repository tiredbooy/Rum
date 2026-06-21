package config

import "testing"

// TestDisarmDestructivePostDownload guards B2: shutdown/sleep/close must be
// reset to "none" at startup (re-armed per session), while none/empty and the
// harmless AutoOpenDir flag are left alone.
func TestDisarmDestructivePostDownload(t *testing.T) {
	for _, a := range []string{"shutdown", "sleep", "close"} {
		var s Setting
		s.PostDownload.Action = a
		if !s.DisarmDestructivePostDownload() {
			t.Errorf("action %q should be disarmed", a)
		}
		if s.PostDownload.Action != "none" {
			t.Errorf("action %q not reset to none, got %q", a, s.PostDownload.Action)
		}
	}

	for _, a := range []string{"none", ""} {
		var s Setting
		s.PostDownload.Action = a
		if s.DisarmDestructivePostDownload() {
			t.Errorf("action %q should not be changed", a)
		}
	}

	var s Setting
	s.PostDownload.Action = "shutdown"
	s.PostDownload.AutoOpenDir = true
	s.DisarmDestructivePostDownload()
	if !s.PostDownload.AutoOpenDir {
		t.Error("AutoOpenDir should be preserved when disarming the power action")
	}
}
