package ftpes

import (
	"crypto/tls"
	"fmt"
	"io"
	"time"

	"github.com/jlaffaye/ftp"
)

const dialTimeout = 30 * time.Second

type client struct {
	conn *ftp.ServerConn
}

func dial(host string, port int, skipTLSVerify bool) (*ftp.ServerConn, error) {
	tlsCfg := &tls.Config{
		InsecureSkipVerify: skipTLSVerify, //nolint:gosec // user-controlled opt-in
		ServerName:         host,
	}

	addr := fmt.Sprintf("%s:%d", host, port)
	conn, err := ftp.Dial(
		addr,
		ftp.DialWithTimeout(dialTimeout),
		ftp.DialWithExplicitTLS(tlsCfg),
	)
	if err != nil {
		return nil, err
	}
	return conn, nil
}

func (c *client) Login(user, pass string) error {
	return c.conn.Login(user, pass)
}

func (c *client) Download(remotePath string, dst io.Writer) error {
	r, err := c.conn.Retr(remotePath)
	if err != nil {
		return fmt.Errorf("RETR %s: %w", remotePath, err)
	}
	defer r.Close()

	if _, err := io.Copy(dst, r); err != nil {
		return fmt.Errorf("read %s: %w", remotePath, err)
	}
	return nil
}

func (c *client) Close() error {
	return c.conn.Quit()
}
