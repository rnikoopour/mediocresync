package sync

import (
	"errors"
	"testing"

	"github.com/rnikoopour/mediocresync/internal/db"
)

func TestFailedTransferError_Message(t *testing.T) {
	err := failedTransferError{failed: 3}
	want := "3 file(s) failed to transfer"
	if err.Error() != want {
		t.Errorf("got %q, want %q", err.Error(), want)
	}
}

func TestPartialTransferError_Message(t *testing.T) {
	err := partialTransferError{completed: 2, failed: 1}
	want := "2 file(s) succeeded, 1 file(s) failed to transfer"
	if err.Error() != want {
		t.Errorf("got %q, want %q", err.Error(), want)
	}
}

// transferStatus mirrors the status-determination branch in executeRun so that
// the same logic can be exercised without a full Engine.
func transferStatus(toCopy int, runErr error) string {
	if runErr == nil {
		if toCopy == 0 {
			return db.RunStatusNothingToSync
		}
		return db.RunStatusCompleted
	}
	var partial partialTransferError
	if errors.As(runErr, &partial) {
		return db.RunStatusPartial
	}
	return db.RunStatusFailed
}

func TestTransferStatus(t *testing.T) {
	tests := []struct {
		name       string
		toCopy     int
		copied     int
		failed     int
		wantStatus string
	}{
		{
			name:       "no files to copy → nothing_to_sync",
			toCopy:     0,
			copied:     0,
			failed:     0,
			wantStatus: db.RunStatusNothingToSync,
		},
		{
			name:       "all succeed → completed",
			toCopy:     3,
			copied:     3,
			failed:     0,
			wantStatus: db.RunStatusCompleted,
		},
		{
			name:       "some succeed some fail → partial",
			toCopy:     3,
			copied:     2,
			failed:     1,
			wantStatus: db.RunStatusPartial,
		},
		{
			name:       "all fail → failed",
			toCopy:     3,
			copied:     0,
			failed:     3,
			wantStatus: db.RunStatusFailed,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var runErr error
			if tt.failed > 0 {
				if tt.copied == 0 {
					runErr = failedTransferError{failed: tt.failed}
				} else {
					runErr = partialTransferError{completed: tt.copied, failed: tt.failed}
				}
			}

			got := transferStatus(tt.toCopy, runErr)
			if got != tt.wantStatus {
				t.Errorf("transferStatus: got %q, want %q", got, tt.wantStatus)
			}
		})
	}
}
