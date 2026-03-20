package sync

import (
	"time"

	"github.com/rnikoopour/mediocresync/internal/db"
	"github.com/rnikoopour/mediocresync/internal/ftpes"
)

const mtimeTolerance = time.Second

// Matches returns true if the remote file's size and mtime match the stored
// state, meaning the file has not changed since it was last copied.
func Matches(state *db.FileState, remote ftpes.RemoteFile) bool {
	if state == nil {
		return false
	}
	if state.SizeBytes != remote.Size {
		return false
	}
	diff := state.MTime.Sub(remote.MTime)
	if diff < 0 {
		diff = -diff
	}
	return diff <= mtimeTolerance
}
