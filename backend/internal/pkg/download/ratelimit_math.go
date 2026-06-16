package download

import "golang.org/x/time/rate"

// bytesPerKB is the conversion factor between kilobytes and bytes used for the
// speed limiter. The previous code multiplied by 102 (a typo for 1024), which
// throttled downloads to ~10% of the configured KB/s.
const bytesPerKB = 1024

// speedLimitBytesPerSec converts a KB/s limit into bytes/s. A non-positive
// limit means "unlimited" and yields 0.
func speedLimitBytesPerSec(limitKB int) int64 {
	if limitKB <= 0 {
		return 0
	}
	return int64(limitKB) * bytesPerKB
}

// newSpeedLimiter builds a rate.Limiter that caps throughput to limitKB
// kilobytes per second. It returns nil when limitKB <= 0 (unlimited).
//
// The burst is set to the per-second byte budget so that a single Read of up to
// downloadBufferSize bytes can always be admitted without the limiter rejecting
// it outright (rate.Limiter.WaitN fails if N exceeds the burst). We therefore
// also ensure the burst is at least one read buffer.
func newSpeedLimiter(limitKB int) *rate.Limiter {
	bps := speedLimitBytesPerSec(limitKB)
	if bps <= 0 {
		return nil
	}
	burst := bps
	if burst < downloadBufferSize {
		burst = downloadBufferSize
	}
	return rate.NewLimiter(rate.Limit(bps), int(burst))
}
