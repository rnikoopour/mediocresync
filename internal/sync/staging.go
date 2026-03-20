package sync

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

const stagingDir = ".mediocresync"

// stagingPath returns the flat staging path for a file during download.
// All in-progress files land in <localDest>/.mediocresync/<filename> regardless
// of their directory depth on the remote.
func stagingPath(localDest, filename string) string {
	return filepath.Join(localDest, stagingDir, filepath.Base(filename))
}

// finalPath computes the local destination path for a remote file, mirroring
// the directory structure relative to the configured remote root.
//
//	remoteRoot:  /exports
//	remotePath:  /exports/reports/2024/jan.csv
//	localDest:   /data/downloads
//	result:      /data/downloads/reports/2024/jan.csv
func finalPath(localDest, remoteRoot, remotePath string) string {
	rel := strings.TrimPrefix(remotePath, remoteRoot)
	rel = strings.TrimPrefix(rel, "/")
	return filepath.Join(localDest, filepath.FromSlash(rel))
}

// atomicMove moves src to dst, creating any intermediate directories needed.
// Both paths must be on the same filesystem (src is always under localDest).
func atomicMove(src, dst string) error {
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return fmt.Errorf("create destination directory: %w", err)
	}
	if err := os.Rename(src, dst); err != nil {
		return fmt.Errorf("rename %s → %s: %w", src, dst, err)
	}
	return nil
}

// ensureStagingDir creates <localDest>/.mediocresync/ if it does not exist.
func ensureStagingDir(localDest string) error {
	dir := filepath.Join(localDest, stagingDir)
	return os.MkdirAll(dir, 0o755)
}
