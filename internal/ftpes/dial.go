package ftpes

import (
	"context"
	"crypto/tls"
	"fmt"
	"io"
	"sync"
	"time"

	"github.com/jlaffaye/ftp"
)

const dialTimeout = 30 * time.Second

type client struct {
	conn *ftp.ServerConn
}

func dial(host string, port int, skipTLSVerify, enableEPSV bool) (*ftp.ServerConn, error) {
	tlsCfg := &tls.Config{
		InsecureSkipVerify: skipTLSVerify, //nolint:gosec // user-controlled opt-in
		ServerName:         host,
	}

	addr := fmt.Sprintf("%s:%d", host, port)
	opts := []ftp.DialOption{
		ftp.DialWithTimeout(dialTimeout),
		ftp.DialWithExplicitTLS(tlsCfg),
		ftp.DialWithDisabledEPSV(!enableEPSV),
	}
	conn, err := ftp.Dial(addr, opts...)
	if err != nil {
		return nil, err
	}
	return conn, nil
}

func (c *client) Login(user, pass string) error {
	return c.conn.Login(user, pass)
}

func (c *client) Download(ctx context.Context, remotePath string, dst io.Writer, offset int64) error {
	var r *ftp.Response
	var err error
	if offset > 0 {
		r, err = c.conn.RetrFrom(remotePath, uint64(offset))
	} else {
		r, err = c.conn.Retr(remotePath)
	}
	if err != nil {
		return fmt.Errorf("RETR %s: %w", remotePath, err)
	}

	// Close the FTP response exactly once (from defer or the ctx watcher below).
	var once sync.Once
	closeResp := func() { once.Do(func() { r.Close() }) }
	defer closeResp()

	// When the context is done, close the FTP data connection so the blocked
	// r.Read() inside io.Copy returns immediately instead of hanging.
	stop := make(chan struct{})
	go func() {
		select {
		case <-ctx.Done():
			closeResp()
		case <-stop:
		}
	}()

	_, copyErr := io.Copy(dst, r)
	close(stop)

	if copyErr != nil {
		return fmt.Errorf("read %s: %w", remotePath, copyErr)
	}
	return ctx.Err()
}

func (c *client) Close() error {
	return c.conn.Quit()
}
