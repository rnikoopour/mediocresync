package ftpes

import (
	"fmt"
	"io"
	"time"
)

// RemoteFile represents a single file on the FTPES server.
type RemoteFile struct {
	Path  string    // full remote path
	Size  int64
	MTime time.Time
}

// Client is the interface the sync engine uses — backed by a real FTPES
// connection in production and a mock in tests.
type Client interface {
	Login(user, pass string) error
	Walk(remotePath string) ([]RemoteFile, error)
	Download(remotePath string, dst io.Writer) error
	Close() error
}

// Dial opens an FTPES connection (plain FTP upgraded to TLS via AUTH TLS).
// Call Login next before any other method.
func Dial(host string, port int, skipTLSVerify bool) (Client, error) {
	conn, err := dial(host, port, skipTLSVerify)
	if err != nil {
		return nil, fmt.Errorf("ftpes dial %s:%d: %w", host, port, err)
	}
	return &client{conn: conn}, nil
}
