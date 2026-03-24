package sync

import (
	"testing"
	"time"

	"github.com/rnikoopour/mediocresync/internal/db"
	"github.com/rnikoopour/mediocresync/internal/ftpes"
)

func TestMatchesNilState(t *testing.T) {
	remote := ftpes.RemoteFile{Path: "/a.csv", Size: 100, MTime: time.Now()}
	if Matches(nil, remote) {
		t.Error("nil state should never match")
	}
}

func TestMatchesExact(t *testing.T) {
	mtime := time.Now().UTC().Truncate(time.Second)
	state := &db.SyncState{SizeBytes: 512, MTime: &mtime}
	remote := ftpes.RemoteFile{Size: 512, MTime: mtime}
	if !Matches(state, remote) {
		t.Error("identical size and mtime should match")
	}
}

func TestMatchesSizeDiffers(t *testing.T) {
	mtime := time.Now().UTC()
	state := &db.SyncState{SizeBytes: 512, MTime: &mtime}
	remote := ftpes.RemoteFile{Size: 513, MTime: mtime}
	if Matches(state, remote) {
		t.Error("different size should not match")
	}
}

func TestMatchesMtimeWithinTolerance(t *testing.T) {
	base := time.Now().UTC()
	state := &db.SyncState{SizeBytes: 100, MTime: &base}
	// Within 1-second tolerance
	remote := ftpes.RemoteFile{Size: 100, MTime: base.Add(999 * time.Millisecond)}
	if !Matches(state, remote) {
		t.Error("mtime within tolerance should match")
	}
}

func TestMatchesMtimeExceedsTolerance(t *testing.T) {
	base := time.Now().UTC()
	state := &db.SyncState{SizeBytes: 100, MTime: &base}
	remote := ftpes.RemoteFile{Size: 100, MTime: base.Add(2 * time.Second)}
	if Matches(state, remote) {
		t.Error("mtime beyond tolerance should not match")
	}
}

func TestMatchesMtimeAtExactBoundary(t *testing.T) {
	base := time.Now().UTC()
	state := &db.SyncState{SizeBytes: 100, MTime: &base}
	remote := ftpes.RemoteFile{Size: 100, MTime: base.Add(time.Second)}
	if !Matches(state, remote) {
		t.Error("mtime at exactly the tolerance boundary should match")
	}
}

func TestMatchesNilMtime(t *testing.T) {
	state := &db.SyncState{SizeBytes: 100, MTime: nil}
	remote := ftpes.RemoteFile{Size: 100, MTime: time.Now()}
	if Matches(state, remote) {
		t.Error("nil MTime should not match")
	}
}
