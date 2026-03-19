package sync

import (
	"context"
	"io"
)

// newPipe returns a connected (reader, writer) pair backed by io.Pipe.
func newPipe() (*io.PipeReader, *io.PipeWriter) {
	return io.Pipe()
}

// copyWithContext copies from src to dst, aborting if ctx is cancelled.
func copyWithContext(ctx context.Context, dst io.Writer, src io.Reader) (int64, error) {
	buf := make([]byte, 32*1024)
	var total int64
	for {
		if ctx.Err() != nil {
			return total, ctx.Err()
		}
		n, readErr := src.Read(buf)
		if n > 0 {
			written, writeErr := dst.Write(buf[:n])
			total += int64(written)
			if writeErr != nil {
				return total, writeErr
			}
		}
		if readErr == io.EOF {
			return total, nil
		}
		if readErr != nil {
			return total, readErr
		}
	}
}
