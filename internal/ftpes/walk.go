package ftpes

import (
	"fmt"
	"path"

	"github.com/jlaffaye/ftp"
)

func (c *client) Walk(remotePath string) ([]RemoteFile, error) {
	var files []RemoteFile
	if err := c.walk(remotePath, &files); err != nil {
		return nil, err
	}
	return files, nil
}

func (c *client) walk(dir string, files *[]RemoteFile) error {
	entries, err := c.conn.List(dir)
	if err != nil {
		return fmt.Errorf("LIST %s: %w", dir, err)
	}

	for _, e := range entries {
		if e.Name == "." || e.Name == ".." {
			continue
		}

		fullPath := path.Join(dir, e.Name)

		switch e.Type {
		case ftp.EntryTypeFile:
			*files = append(*files, RemoteFile{
				Path:  fullPath,
				Size:  int64(e.Size),
				MTime: e.Time,
			})
		case ftp.EntryTypeFolder:
			if err := c.walk(fullPath, files); err != nil {
				return err
			}
		// skip symlinks and other types
		}
	}
	return nil
}
