package config

import "testing"

func TestNotifyWhenHiddenDefaultsOnWhenKeyAbsent(t *testing.T) {
	writeSettingsFile(t, `{"out_dir":"/tmp"}`)
	var s Setting
	if err := s.LoadSettingMetadata(); err != nil {
		t.Fatal(err)
	}
	if !s.NotifyWhenHidden {
		t.Fatal("notify_when_hidden should default to true")
	}
}

func TestNotifyWhenHiddenExplicitFalseSurvivesAndPatches(t *testing.T) {
	writeSettingsFile(t, `{"out_dir":"/tmp","notify_when_hidden":false}`)
	var s Setting
	if err := s.LoadSettingMetadata(); err != nil {
		t.Fatal(err)
	}
	if s.NotifyWhenHidden {
		t.Fatal("explicit false was overwritten")
	}
	on := true
	s.Update(SettingReq{NotifyWhenHidden: &on})
	if !s.NotifyWhenHidden {
		t.Fatal("PATCH did not apply notify_when_hidden")
	}
}
