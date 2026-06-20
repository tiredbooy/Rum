package download

import (
	"context"
	"io"

	"golang.org/x/time/rate"
)

// downloadBufferSize is the read/write chunk for transfers. 128 KiB cuts syscalls
// ~4x versus the old 32 KiB (better throughput and lower CPU on fast links) while
// staying small enough that it doubles as the rate-limiter burst floor without
// making a speed-limited download visibly bursty. NOTE: this is NOT what governs
// UI smoothness — progress frames are time-throttled separately (see runDownload),
// and the frontend interpolates the bar between them.
const downloadBufferSize = 128 * 1024

type rateLimitedReader struct {
	reader  io.ReadCloser
	limiter *rate.Limiter
	ctx     context.Context
}

func (r *rateLimitedReader) Read(p []byte) (int, error) {
	n, err := r.reader.Read(p)
	if n > 0 {
		if waitErr := r.limiter.WaitN(r.ctx, n); waitErr != nil {
			return n, waitErr
		}
	}
	return n, err
}

func (r *rateLimitedReader) Close() error {
	return r.reader.Close()
}
