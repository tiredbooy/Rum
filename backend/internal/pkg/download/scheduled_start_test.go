package download

import (
	"testing"
	"time"
)

// TestShouldDeferAutoStart locks the create-side guard for the bug where a
// per-download "scheduled start" was ignored whenever AutoStart was on (the
// default), so a future-dated download started immediately.
func TestShouldDeferAutoStart(t *testing.T) {
	now := time.Now()
	cases := []struct {
		name    string
		startAt time.Time
		want    bool
	}{
		{"zero StartAt starts now", time.Time{}, false},
		{"past StartAt starts now", now.Add(-time.Hour), false},
		{"future StartAt is deferred", now.Add(time.Hour), true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			j := &Job{ID: "x"}
			j.SetStartAt(tc.startAt)
			if got := ShouldDeferAutoStart(j); got != tc.want {
				t.Fatalf("ShouldDeferAutoStart(startAt=%v) = %v, want %v", tc.startAt, got, tc.want)
			}
		})
	}
}
