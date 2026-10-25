package config

import "testing"

func TestParseClock(t *testing.T) {
	good := map[string]int{"00:00": 0, "02:00": 120, "07:30": 450, "23:59": 1439}
	for in, want := range good {
		got, ok := ParseClock(in)
		if !ok || got != want {
			t.Errorf("ParseClock(%q) = %d,%v want %d,true", in, got, ok, want)
		}
	}
	for _, in := range []string{"", "2:00", "24:00", "12:60", "ab:cd", "12:5", "12:00:00", " 02:00"} {
		if _, ok := ParseClock(in); ok {
			t.Errorf("ParseClock(%q) accepted, want rejected", in)
		}
	}
}

func TestNightPlanProblems(t *testing.T) {
	ok := NightPlan{Enabled: true, Start: "02:00", End: "07:00", CapGB: 5}
	if p := ok.Problems(); p != nil {
		t.Fatalf("valid plan reported problems: %v", p)
	}
	cross := NightPlan{Enabled: true, Start: "23:00", End: "06:00"}
	if p := cross.Problems(); p != nil {
		t.Fatalf("midnight-crossing plan must be valid: %v", p)
	}
	cases := map[string]NightPlan{
		"start":  {Start: "25:00", End: "07:00"},
		"end":    {Start: "02:00", End: "7"},
		"window": {Start: "02:00", End: "02:00"},
		"cap_gb": {Start: "02:00", End: "07:00", CapGB: -1},
	}
	for field, p := range cases {
		if _, bad := p.Problems()[field]; !bad {
			t.Errorf("plan %+v: want a problem on %q, got %v", p, field, p.Problems())
		}
	}
	huge := NightPlan{Start: "02:00", End: "07:00", CapGB: 1e9}
	if _, bad := huge.Problems()["cap_gb"]; !bad {
		t.Errorf("absurd cap accepted")
	}
}

func TestNightPlanSanitizeFillsDefaults(t *testing.T) {
	// A settings file written before the feature carries no night_plan at all.
	p := NightPlan{}
	p.Sanitize()
	if p.Enabled || p.Start != "02:00" || p.End != "07:00" || !p.StartAtOpen || !p.PauseAtClose {
		t.Fatalf("zero plan sanitized to %+v", p)
	}
	// A hand-edited bad value falls back without touching the rest.
	q := NightPlan{Enabled: true, Start: "junk", End: "06:00", CapGB: -3}
	q.Sanitize()
	if !q.Enabled || q.Start != "02:00" || q.End != "06:00" || q.CapGB != 0 {
		t.Fatalf("sanitized = %+v", q)
	}
	// Start == End would be an empty window: repaired, not kept.
	r := NightPlan{Start: "03:00", End: "03:00"}
	r.Sanitize()
	if r.Problems() != nil {
		t.Fatalf("sanitized plan still invalid: %+v", r)
	}
}
