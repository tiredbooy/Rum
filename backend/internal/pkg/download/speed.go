package download

import "time"

const (
	// speedSampleInterval is the minimum real elapsed time between speed
	// recomputations. The engine reports progress on every buffer (tens of KB)
	// from every segment, so consecutive callbacks can be microseconds apart;
	// dividing one buffer by a microsecond yields absurd instantaneous rates
	// (hundreds of MB/s for an 8 MB/s download). Recomputing only once per real
	// window makes the reported rate reflect actual throughput.
	speedSampleInterval = 500 * time.Millisecond
	// speedAlpha is the EMA weight applied to each new windowed sample (higher =
	// more responsive, lower = smoother).
	speedAlpha = 0.3
)

// speedEstimator turns a stream of cumulative byte counts into a smoothed
// bytes/sec estimate, sampled over a real time window. It is intentionally a tiny,
// self-contained unit so it can be unit-tested with synthetic timestamps. Not safe
// for concurrent use: each running job owns one and feeds it from its single,
// serialized progress callback.
type speedEstimator struct {
	lastTime       time.Time
	lastDownloaded int64
	smooth         float64
}

// sample feeds a cumulative downloaded byte count observed at time now and returns
// the current smoothed speed in bytes/sec. It only recomputes once at least
// speedSampleInterval has elapsed since the previous sample, so a burst of
// near-simultaneous callbacks cannot inflate the rate; between windows it returns
// the last smoothed value unchanged.
func (e *speedEstimator) sample(downloaded int64, now time.Time) float64 {
	if e.lastTime.IsZero() {
		e.lastTime = now
		e.lastDownloaded = downloaded
		return e.smooth
	}
	elapsed := now.Sub(e.lastTime).Seconds()
	if elapsed < speedSampleInterval.Seconds() {
		return e.smooth
	}
	instant := float64(downloaded-e.lastDownloaded) / elapsed
	if instant < 0 {
		// A counter that went backwards (e.g. a restart-from-zero) must not produce
		// a negative speed; treat it as idle for this window.
		instant = 0
	}
	if e.smooth == 0 {
		e.smooth = instant
	} else {
		e.smooth = speedAlpha*instant + (1-speedAlpha)*e.smooth
	}
	e.lastTime = now
	e.lastDownloaded = downloaded
	return e.smooth
}
