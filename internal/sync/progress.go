package sync

import (
	"io"
	"sync/atomic"
	"time"
)

const progressInterval = 250 * time.Millisecond

// progressReader wraps an io.Reader and calls callback with the running byte
// count after each read, rate-limited to at most once per progressInterval.
type progressReader struct {
	r        io.Reader
	total    int64
	n        atomic.Int64
	lastEmit time.Time
	callback func(bytesRead int64)
}

func newProgressReader(r io.Reader, total int64, callback func(int64)) *progressReader {
	return &progressReader{
		r:        r,
		total:    total,
		lastEmit: time.Now(),
		callback: callback,
	}
}

func (p *progressReader) Read(buf []byte) (int, error) {
	n, err := p.r.Read(buf)
	if n > 0 {
		current := p.n.Add(int64(n))
		if time.Since(p.lastEmit) >= progressInterval || err == io.EOF {
			p.lastEmit = time.Now()
			p.callback(current)
		}
	}
	return n, err
}
