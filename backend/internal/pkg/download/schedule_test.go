package download

import (
	"context"
	"testing"
	"time"

	"github.com/tiredbooy/Rum/backend/internal/pkg/config"
)

// at builds a local time on a fixed date at the given hour/weekday. We choose a
// reference Monday (2024-01-01 is a Monday) and offset by weekday.
func atWeekdayHour(weekday time.Weekday, hour int) time.Time {
	// 2024-01-01 is a Monday (weekday 1). Compute the day-of-month for the wanted
	// weekday within that same week (Sun..Sat => 31 Dec .. 6 Jan).
	// Sunday=0 -> 2023-12-31, Monday=1 -> 2024-01-01, ... Saturday=6 -> 2024-01-06.
	base := time.Date(2023, 12, 31, 0, 0, 0, 0, time.Local) // Sunday
	d := base.AddDate(0, 0, int(weekday))
	return time.Date(d.Year(), d.Month(), d.Day(), hour, 0, 0, 0, time.Local)
}

func TestActiveLimitKBps(t *testing.T) {
	fallback := 9999

	tests := []struct {
		name  string
		rules []config.SpeedRule
		now   time.Time
		want  int
	}{
		{
			name:  "no rules returns fallback",
			rules: nil,
			now:   atWeekdayHour(time.Wednesday, 3),
			want:  fallback,
		},
		{
			name:  "in-window returns rule limit",
			rules: []config.SpeedRule{{StartHour: 0, EndHour: 8, LimitKBps: 500}},
			now:   atWeekdayHour(time.Wednesday, 3),
			want:  500,
		},
		{
			name:  "out-of-window returns fallback",
			rules: []config.SpeedRule{{StartHour: 0, EndHour: 8, LimitKBps: 500}},
			now:   atWeekdayHour(time.Wednesday, 12),
			want:  fallback,
		},
		{
			name:  "day-restricted rule applies on its day",
			rules: []config.SpeedRule{{StartHour: 0, EndHour: 23, LimitKBps: 300, Days: []int{1}}}, // Monday only
			now:   atWeekdayHour(time.Monday, 10),
			want:  300,
		},
		{
			name:  "day-restricted rule ignored off its day",
			rules: []config.SpeedRule{{StartHour: 0, EndHour: 23, LimitKBps: 300, Days: []int{1}}}, // Monday only
			now:   atWeekdayHour(time.Tuesday, 10),
			want:  fallback,
		},
		{
			name: "overlapping rules pick most restrictive non-zero",
			rules: []config.SpeedRule{
				{StartHour: 0, EndHour: 23, LimitKBps: 800},
				{StartHour: 8, EndHour: 18, LimitKBps: 200},
			},
			now:  atWeekdayHour(time.Wednesday, 10),
			want: 200,
		},
		{
			name: "zero-limit rule does not override a non-zero one",
			rules: []config.SpeedRule{
				{StartHour: 0, EndHour: 23, LimitKBps: 0}, // unlimited window
				{StartHour: 8, EndHour: 18, LimitKBps: 400},
			},
			now:  atWeekdayHour(time.Wednesday, 10),
			want: 400,
		},
		{
			name:  "wrap-around window matches late evening",
			rules: []config.SpeedRule{{StartHour: 22, EndHour: 6, LimitKBps: 150}},
			now:   atWeekdayHour(time.Wednesday, 23),
			want:  150,
		},
		{
			name:  "wrap-around window matches early morning",
			rules: []config.SpeedRule{{StartHour: 22, EndHour: 6, LimitKBps: 150}},
			now:   atWeekdayHour(time.Wednesday, 2),
			want:  150,
		},
		{
			name:  "wrap-around window excludes midday",
			rules: []config.SpeedRule{{StartHour: 22, EndHour: 6, LimitKBps: 150}},
			now:   atWeekdayHour(time.Wednesday, 12),
			want:  fallback,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := activeLimitKBps(tc.rules, tc.now, fallback)
			if got != tc.want {
				t.Fatalf("activeLimitKBps(%v) = %d, want %d", tc.name, got, tc.want)
			}
		})
	}
}

func TestScheduleControllerLeakFreeStop(t *testing.T) {
	// A controller started and stopped must not leak its ticker goroutine.
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	gov := NewSpeedGovernor(0)
	opt := &Options{Parallel: 1, Connections: 1, Out: t.TempDir(), Downloader: NewDownloader("t", "")}
	m := NewJobManager(opt)
	defer m.Shutdown()

	var setting config.Setting
	setting.SpeedLimitKB = 1000
	ctrl := NewScheduleController(m, gov, setting)

	ctx, cancel := context.WithCancel(context.Background())
	ctrl.Start(ctx)
	// Let it tick at least once via its immediate initial apply.
	time.Sleep(20 * time.Millisecond)
	cancel()
	ctrl.Stop()
	// If Stop returns, the goroutine has exited (Stop waits on it).
}
