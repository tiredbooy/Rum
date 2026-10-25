package config

import "testing"

func TestPreventSleepDefaultsOnWhenKeyAbsent(t *testing.T) {
	writeSettingsFile(t, `{"out_dir":"/tmp"}`)
	var s Setting
	if err := s.LoadSettingMetadata(); err != nil {
		t.Fatal(err)
	}
	if !s.PreventSleep {
		t.Fatal("prevent_sleep should default to true")
	}
}

func TestPreventSleepExplicitFalseSurvivesAndPatches(t *testing.T) {
	writeSettingsFile(t, `{"out_dir":"/tmp","prevent_sleep":false}`)
	var s Setting
	if err := s.LoadSettingMetadata(); err != nil {
		t.Fatal(err)
	}
	if s.PreventSleep {
		t.Fatal("explicit false was overwritten")
	}
	on := true
	s.Update(SettingReq{PreventSleep: &on})
	if !s.PreventSleep {
		t.Fatal("PATCH did not apply prevent_sleep")
	}
}
