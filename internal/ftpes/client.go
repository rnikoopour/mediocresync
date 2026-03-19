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

// DirEntry is one item returned by a shallow directory listing.
type DirEntry struct {
	Name  string
	Path  string
	IsDir bool
}

// Client is the interface the sync engine uses — backed by a real FTPES
// connection in production and a mock in tests.
type Client interface {
	Login(user, pass string) error
	List(remotePath string) ([]DirEntry, error)
	Walk(remotePath string) ([]RemoteFile, error)
	WalkWithProgress(remotePath string, progress func(files, dirs int)) ([]RemoteFile, error)
	Download(remotePath string, dst io.Writer) error
	Close() error
}

// Dial opens an FTPES connection (plain FTP upgraded to TLS via AUTH TLS).
// Call Login next before any other method.
func Dial(host string, port int, skipTLSVerify, enableEPSV bool) (Client, error) {
	conn, err := dial(host, port, skipTLSVerify, enableEPSV)
	if err != nil {
		return nil, fmt.Errorf("ftpes dial %s:%d: %w", host, port, err)
	}
	return &client{conn: conn}, nil
}
