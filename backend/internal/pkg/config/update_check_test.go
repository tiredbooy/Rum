package config

import "testing"

func TestCheckForUpdatesDefaultsOnWhenKeyAbsent(t *testing.T) {
	writeSettingsFile(t, `{"out_dir":"/tmp"}`)
	var s Setting
	if err := s.LoadSettingMetadata(); err != nil {
		t.Fatal(err)
	}
	if !s.CheckForUpdates {
		t.Fatal("check_for_updates should default to true")
	}
	if len(s.UpdateMirrors) != 0 {
		t.Fatalf("mirrors should default to empty (built-in list), got %v", s.UpdateMirrors)
	}
}

func TestCheckForUpdatesExplicitFalseSurvivesAndPatches(t *testing.T) {
	writeSettingsFile(t, `{"out_dir":"/tmp","check_for_updates":false}`)
	var s Setting
	if err := s.LoadSettingMetadata(); err != nil {
		t.Fatal(err)
	}
	if s.CheckForUpdates {
		t.Fatal("explicit false was overwritten")
	}
	on := true
	mirrors := []string{"https://example.com/"}
	if err := s.Update(SettingReq{CheckForUpdates: &on, UpdateMirrors: &mirrors}); err != nil {
		t.Fatal(err)
	}
	var got Setting
	if err := got.LoadSettingMetadata(); err != nil {
		t.Fatal(err)
	}
	if !got.CheckForUpdates || len(got.UpdateMirrors) != 1 || got.UpdateMirrors[0] != "https://example.com/" {
		t.Fatalf("not persisted: %+v %v", got.CheckForUpdates, got.UpdateMirrors)
	}
}
