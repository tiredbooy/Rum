package download

import (
	"context"
	"io"

	"golang.org/x/time/rate"
)

const downloadBufferSize = 32 * 1024

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
