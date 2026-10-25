package config

import (
	"fmt"
	"math"
	"strconv"
)

// maxNightCapGB bounds the optional per-window data cap. It only exists to
// reject typos and NaN; 100 TB is far beyond any home plan.
const maxNightCapGB = 100000

// NightPlan is the daily "night internet" window: Iranian ISPs sell cheap or
// free night traffic (for example 02:00-07:00), and jobs assigned to the night
// queue run only inside it.
//
// Start and End are local wall-clock "HH:MM" strings. End may be earlier than
// Start, which means the window crosses midnight (23:00-06:00). The time zone is
// the machine's local one at evaluation time, so DST changes need no stored
// offset.
type NightPlan struct {
	Enabled bool   `json:"enabled"`
	Start   string `json:"start"` // "HH:MM"
	End     string `json:"end"`   // "HH:MM"
	// StartAtOpen starts queued and schedule-paused night jobs when the window
	// opens (and on launch when the app starts inside it).
	StartAtOpen bool `json:"start_at_open"`
	// PauseAtClose pauses running night jobs when the window closes. Pausing
	// keeps the partial data, so the job resumes from where it stopped.
	PauseAtClose bool `json:"pause_at_close"`
	// CapGB is an optional data cap per window in GB (0 = no cap). Night jobs are
	// paused for the rest of the window once it is spent.
	CapGB float64 `json:"cap_gb"`
}

// DefaultNightPlan returns the plan a fresh install starts from: off, with the
// usual 02:00-07:00 window and both automatic actions on.
func DefaultNightPlan() NightPlan {
	return NightPlan{Start: "02:00", End: "07:00", StartAtOpen: true, PauseAtClose: true}
}

// ParseClock parses a strict "HH:MM" 24-hour time into minutes after midnight.
func ParseClock(s string) (int, bool) {
	if len(s) != 5 || s[2] != ':' {
		return 0, false
	}
	h, err1 := strconv.Atoi(s[0:2])
	m, err2 := strconv.Atoi(s[3:5])
	if err1 != nil || err2 != nil || s[0] == '+' || s[0] == '-' || s[3] == '+' || s[3] == '-' {
		return 0, false
	}
	if h < 0 || h > 23 || m < 0 || m > 59 {
		return 0, false
	}
	return h*60 + m, true
}

// Problems reports field -> message for every invalid field, or nil when the
// plan is valid. The API rejects a plan with problems; Sanitize repairs one read
// from a hand-edited file. Frontend validation mirrors these rules.
func (p NightPlan) Problems() map[string]string {
	out := map[string]string{}
	s, okS := ParseClock(p.Start)
	e, okE := ParseClock(p.End)
	if !okS {
		out["start"] = "must be a time like 02:00"
	}
	if !okE {
		out["end"] = "must be a time like 07:00"
	}
	if okS && okE && s == e {
		out["window"] = "start and end must differ"
	}
	if math.IsNaN(p.CapGB) || p.CapGB < 0 || p.CapGB > maxNightCapGB {
		out["cap_gb"] = fmt.Sprintf("must be 0 (no cap) to %d", maxNightCapGB)
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

// Sanitize repairs a plan loaded from disk so the engine never sees an invalid
// window. A plan that was never configured (no times at all) gets the full
// defaults; otherwise only the broken fields are replaced.
func (p *NightPlan) Sanitize() {
	if p.Start == "" && p.End == "" && !p.StartAtOpen && !p.PauseAtClose {
		d := DefaultNightPlan()
		d.Enabled, d.CapGB = p.Enabled, p.CapGB
		*p = d
	}
	def := DefaultNightPlan()
	if _, ok := ParseClock(p.Start); !ok {
		p.Start = def.Start
	}
	if _, ok := ParseClock(p.End); !ok {
		p.End = def.End
	}
	if p.Start == p.End {
		p.End = def.End
		if p.Start == p.End {
			p.End = "08:00"
		}
	}
	if math.IsNaN(p.CapGB) || p.CapGB < 0 {
		p.CapGB = 0
	}
	if p.CapGB > maxNightCapGB {
		p.CapGB = maxNightCapGB
	}
}
