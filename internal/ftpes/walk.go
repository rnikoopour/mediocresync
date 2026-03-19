package ftpes

import (
	"fmt"
	"path"
	"strings"
	"time"

	"github.com/jlaffaye/ftp"
)

const listMaxAttempts = 3
const listRetryDelay = 500 * time.Millisecond

// listDir wraps conn.List with retries for transient 425 data-connection errors.
func (c *client) listDir(remotePath string) ([]*ftp.Entry, error) {
	var lastErr error
	for attempt := 1; attempt <= listMaxAttempts; attempt++ {
		entries, err := c.conn.List(remotePath)
		if err == nil {
			return entries, nil
		}
		lastErr = err
		if !strings.Contains(err.Error(), "425") {
			break
		}
		if attempt < listMaxAttempts {
			time.Sleep(listRetryDelay)
		}
	}
	return nil, lastErr
}

func (c *client) List(remotePath string) ([]DirEntry, error) {
	entries, err := c.listDir(remotePath)
	if err != nil {
		return nil, fmt.Errorf("LIST %s: %w", remotePath, err)
	}

	out := make([]DirEntry, 0, len(entries))
	for _, e := range entries {
		if e.Name == "." || e.Name == ".." {
			continue
		}
		out = append(out, DirEntry{
			Name:  e.Name,
			Path:  path.Join(remotePath, e.Name),
			IsDir: e.Type == ftp.EntryTypeFolder,
		})
	}
	return out, nil
}

func (c *client) Walk(remotePath string) ([]RemoteFile, error) {
	var files []RemoteFile
	if err := c.walk(remotePath, &files); err != nil {
		return nil, err
	}
	return files, nil
}

func (c *client) WalkWithProgress(remotePath string, shouldDescend func(dir string) bool, progress func(files, dirs int)) ([]RemoteFile, error) {
	var result []RemoteFile
	var nFiles, nDirs int
	if err := c.walkProgress(remotePath, shouldDescend, &result, &nFiles, &nDirs, progress); err != nil {
		return nil, err
	}
	return result, nil
}

func (c *client) walkProgress(dir string, shouldDescend func(dir string) bool, files *[]RemoteFile, nFiles, nDirs *int, progress func(files, dirs int)) error {
	entries, err := c.listDir(dir)
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
			*nFiles++
			progress(*nFiles, *nDirs)
		case ftp.EntryTypeFolder:
			*nDirs++
			progress(*nFiles, *nDirs)
			if shouldDescend == nil || shouldDescend(fullPath) {
				if err := c.walkProgress(fullPath, shouldDescend, files, nFiles, nDirs, progress); err != nil {
					return err
				}
			}
		}
	}
	return nil
}

func (c *client) walk(dir string, files *[]RemoteFile) error {
	entries, err := c.listDir(dir)
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
