package download

import (
	"context"
	"errors"
	"sort"
	"sync"
	"sync/atomic"
	"time"
)

// Speed-based segment restart. Throttled links often cap each TCP connection
// rather than the whole host, so a segment crawling far below its siblings is
// closed and reopened from its current offset; the new connection frequently
// escapes the throttle. This is distinct from the stall watchdog (stall.go):
// a stall is zero bytes, "slow" is some bytes but far below the siblings.

// errSlowRestart is the cancel cause used to drop one segment connection.
var errSlowRestart = errors.New("segment too slow, reconnecting")

// slowConfig tunes the slow-segment detector.
type slowConfig struct {
	Window      time.Duration // sliding window used to measure a segment's speed
	Sustain     time.Duration // how long a segment must stay slow (doubles per restart)
	Tick        time.Duration // evaluation period
	Ratio       float64       // slow means speed < Ratio * median of siblings
	FloorBps    float64       // sibling median must exceed this (bytes/s) to matter
	MaxRestarts int           // per-segment cap
}

func defaultSlowConfig() slowConfig {
	return slowConfig{
		Window:      10 * time.Second,
		Sustain:     15 * time.Second,
		Tick:        time.Second,
		Ratio:       0.2,
		FloorBps:    256 * 1024,
		MaxRestarts: 5,
	}
}

// isSlowSegment is the restart decision for one segment: speeds[i] is segment
// i's windowed speed in bytes/s, or < 0 when it cannot be measured (idle, new
// connection, finished). A segment with zero bytes is a stall, not slow.
func isSlowSegment(speeds []float64, idx int, cfg slowConfig) bool {
	if idx < 0 || idx >= len(speeds) || speeds[idx] <= 0 {
		return false
	}
	var sib []float64
	for i, s := range speeds {
		if i != idx && s >= 0 {
			sib = append(sib, s)
		}
	}
	if len(sib) == 0 {
		return false
	}
	sort.Float64s(sib)
	med := sib[len(sib)/2]
	if len(sib)%2 == 0 {
		med = (sib[len(sib)/2-1] + sib[len(sib)/2]) / 2
	}
	return med > cfg.FloorBps && speeds[idx] < cfg.Ratio*med
}

type slowSample struct {
	at    time.Time
	bytes int64
}

type slowSeg struct {
	done      *atomic.Int64
	cancel    context.CancelCauseFunc
	active    bool
	connAt    time.Time
	samples   []slowSample
	slowSince time.Time
	restarts  int
}

// slowMonitor watches all segments of one download pass.
type slowMonitor struct {
	cfg     slowConfig
	limited func() bool // a speed limit is shaping throughput: never restart
	now     func() time.Time

	mu   sync.Mutex
	segs []*slowSeg
}

func newSlowMonitor(cfg slowConfig, done []atomic.Int64, limited func() bool) *slowMonitor {
	m := &slowMonitor{cfg: cfg, limited: limited, now: time.Now, segs: make([]*slowSeg, len(done))}
	for i := range done {
		m.segs[i] = &slowSeg{done: &done[i]}
	}
	return m
}

// attach registers a freshly opened connection for segment i.
func (m *slowMonitor) attach(i int, cancel context.CancelCauseFunc) {
	if m == nil {
		return
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	s := m.segs[i]
	s.cancel, s.active, s.connAt = cancel, true, m.now()
	s.samples = []slowSample{{m.now(), s.done.Load()}}
	s.slowSince = time.Time{}
}

// detach marks segment i as having no live connection.
func (m *slowMonitor) detach(i int) {
	if m == nil {
		return
	}
	m.mu.Lock()
	m.segs[i].active, m.segs[i].cancel = false, nil
	m.mu.Unlock()
}

// speed returns the windowed speed or -1 when not yet measurable.
func (s *slowSeg) speed(now time.Time, window time.Duration) float64 {
	cur := s.done.Load()
	s.samples = append(s.samples, slowSample{now, cur})
	// Keep exactly one sample at or before the window start as the baseline.
	cut := 0
	for i, sm := range s.samples {
		if !sm.at.After(now.Add(-window)) {
			cut = i
		}
	}
	s.samples = s.samples[cut:]
	base := s.samples[0]
	if !s.active || now.Sub(base.at) < window || now.Sub(s.connAt) < window {
		return -1
	}
	return float64(cur-base.bytes) / now.Sub(base.at).Seconds()
}

// evaluate runs one detection tick and restarts at most the segments that have
// stayed slow long enough. Exposed to tests with an injectable clock.
func (m *slowMonitor) evaluate() {
	m.mu.Lock()
	defer m.mu.Unlock()
	now := m.now()
	speeds := make([]float64, len(m.segs))
	for i, s := range m.segs {
		speeds[i] = s.speed(now, m.cfg.Window)
	}
	if m.limited != nil && m.limited() {
		for _, s := range m.segs {
			s.slowSince = time.Time{}
		}
		return
	}
	for i, s := range m.segs {
		if !isSlowSegment(speeds, i, m.cfg) || s.restarts >= m.cfg.MaxRestarts {
			s.slowSince = time.Time{}
			continue
		}
		if s.slowSince.IsZero() {
			s.slowSince = now
			continue
		}
		need := m.cfg.Sustain << uint(s.restarts) // backoff: 1x, 2x, 4x ...
		if max := 8 * m.cfg.Sustain; need > max {
			need = max
		}
		if now.Sub(s.slowSince) >= need && s.cancel != nil {
			s.restarts++
			s.slowSince = time.Time{}
			DebugLog("segmented: restarting slow segment")
			s.cancel(errSlowRestart)
			s.active = false
		}
	}
}

// run evaluates until ctx ends.
func (m *slowMonitor) run(ctx context.Context) {
	t := time.NewTicker(m.cfg.Tick)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			m.evaluate()
		}
	}
}
