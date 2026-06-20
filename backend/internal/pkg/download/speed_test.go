package download

import (
	"testing"
	"time"
)

// Reproduces the reported bug: the engine fires the progress callback on every
// ~32 KiB buffer from 8 segments, so callbacks arrive in micro-bursts microseconds
// apart. The OLD per-callback delta (Δbytes / Δtime between consecutive callbacks)
// computed ~hundreds of MB/s for an ~8 MB/s download. The windowed estimator must
// stay near the real throughput and never spike.
func TestSpeedEstimatorIgnoresBurstyCallbacks(t *testing.T) {
	const buf = int64(32 << 10)                                       // 32 KiB per callback
	const perBurst = 8                                                // 8 segments deliver ~together
	const burstGap = 31 * time.Millisecond                            // wall time between bursts
	realRate := float64(perBurst) * float64(buf) / burstGap.Seconds() // ~8 MiB/s

	est := &speedEstimator{}
	now := time.Unix(100, 0)
	var downloaded int64
	var maxReported float64

	// ~7.75s of transfer, delivered as bursts of 8 buffers 7µs apart, then a 31ms gap.
	for b := 0; b < 250; b++ {
		for s := 0; s < perBurst; s++ {
			downloaded += buf
			now = now.Add(7 * time.Microsecond) // near-simultaneous within the burst
			if sp := est.sample(downloaded, now); sp > maxReported {
				maxReported = sp
			}
		}
		now = now.Add(burstGap)
	}

	// Must never blow up to the hundreds-of-MB/s the bug produced. The old inline
	// per-callback math would report multi-GB/s inside each burst.
	if maxReported > 2*realRate {
		t.Fatalf("speed spiked to %.0f MiB/s; real ~%.1f MiB/s — windowing failed",
			maxReported/(1<<20), realRate/(1<<20))
	}
	// And the smoothed estimate should converge near the real rate, not near zero.
	if est.smooth < 0.5*realRate || est.smooth > 1.7*realRate {
		t.Fatalf("converged speed %.2f MiB/s not near real %.2f MiB/s",
			est.smooth/(1<<20), realRate/(1<<20))
	}
}

// Evenly-spaced callbacks (no bursts) must report the true rate — confirms the
// window doesn't distort a well-behaved stream.
func TestSpeedEstimatorSteadyStream(t *testing.T) {
	const buf = int64(64 << 10)
	gap := 8 * time.Millisecond // 64 KiB / 8ms = 8 MiB/s
	realRate := float64(buf) / gap.Seconds()

	est := &speedEstimator{}
	now := time.Unix(0, 0)
	var downloaded int64
	for i := 0; i < 1000; i++ {
		downloaded += buf
		now = now.Add(gap)
		est.sample(downloaded, now)
	}
	if est.smooth < 0.8*realRate || est.smooth > 1.2*realRate {
		t.Fatalf("steady speed %.2f MiB/s not near real %.2f MiB/s",
			est.smooth/(1<<20), realRate/(1<<20))
	}
}
