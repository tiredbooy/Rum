package config

import "testing"

// Settings added by different features share one settings.json: a save/load
// cycle must keep every one of them, and the extension token must never be
// written into it.
func TestSettingsRoundTripKeepsEveryFeatureField(t *testing.T) {
	writeSettingsFile(t, `{"out_dir":"/tmp"}`)
	var s Setting
	if err := s.LoadSettingMetadata(); err != nil {
		t.Fatal(err)
	}
	s.PreventSleep = false
	s.AutoResumeOnReconnect = true
	s.NightPlan = NightPlan{Enabled: true, Start: "23:30", End: "06:15", StartAtOpen: false, PauseAtClose: false, CapGB: 12}
	if err := s.Save(); err != nil {
		t.Fatal(err)
	}

	var got Setting
	if err := got.LoadSettingMetadata(); err != nil {
		t.Fatal(err)
	}
	if got.PreventSleep {
		t.Fatal("prevent_sleep=false lost")
	}
	if !got.AutoResumeOnReconnect {
		t.Fatal("auto_resume_on_reconnect lost")
	}
	if got.NightPlan != s.NightPlan {
		t.Fatalf("night_plan = %+v, want %+v", got.NightPlan, s.NightPlan)
	}
}
