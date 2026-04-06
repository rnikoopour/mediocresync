package db

import (
	"database/sql"
	"fmt"
	"time"

	"github.com/google/uuid"
)

type TransferRepository struct {
	db *sql.DB
}

func NewTransferRepository(db *sql.DB) *TransferRepository {
	return &TransferRepository{db: db}
}

func (r *TransferRepository) Create(t *Transfer) error {
	t.ID = uuid.New().String()

	_, err := r.db.Exec(
		`INSERT INTO transfers (id, run_id, remote_path, local_path, size_bytes, bytes_xferred, status, previous_commit_hash, current_commit_hash)
		 VALUES (?, ?, ?, ?, ?, 0, ?, ?, ?)`,
		t.ID, t.RunID, t.RemotePath, t.LocalPath, t.SizeBytes, t.Status, t.PreviousCommitHash, t.CurrentCommitHash,
	)
	if err != nil {
		return fmt.Errorf("insert transfer: %w", err)
	}
	return nil
}

func (r *TransferRepository) UpdateProgress(id string, bytesXferred int64) error {
	_, err := r.db.Exec(
		`UPDATE transfers SET bytes_xferred=?, status='in_progress', started_at=COALESCE(started_at, ?) WHERE id=?`,
		bytesXferred, formatTime(time.Now().UTC()), id,
	)
	return err
}


// CreateBatch inserts multiple transfer records in a single transaction.
func (r *TransferRepository) CreateBatch(transfers []*Transfer) error {
	if len(transfers) == 0 {
		return nil
	}
	tx, err := r.db.Begin()
	if err != nil {
		return fmt.Errorf("begin batch insert: %w", err)
	}
	defer tx.Rollback() //nolint:errcheck
	stmt, err := tx.Prepare(
		`INSERT INTO transfers (id, run_id, remote_path, local_path, size_bytes, bytes_xferred, status, previous_commit_hash, current_commit_hash)
		 VALUES (?, ?, ?, ?, ?, 0, ?, ?, ?)`,
	)
	if err != nil {
		return fmt.Errorf("prepare batch insert: %w", err)
	}
	defer stmt.Close()
	for _, t := range transfers {
		t.ID = uuid.New().String()
		if _, err := stmt.Exec(t.ID, t.RunID, t.RemotePath, t.LocalPath, t.SizeBytes, t.Status, t.PreviousCommitHash, t.CurrentCommitHash); err != nil {
			return fmt.Errorf("insert transfer: %w", err)
		}
	}
	return tx.Commit()
}

func (r *TransferRepository) UpdateCurrentCommitHash(id, hash string) error {
	_, err := r.db.Exec(`UPDATE transfers SET current_commit_hash=? WHERE id=?`, hash, id)
	return err
}

func (r *TransferRepository) UpdateStatus(id, status string, errMsg *string, durationMs *int64) error {
	var finishedAt *string
	if status == TransferStatusDone || status == TransferStatusFailed || status == TransferStatusSkipped || status == TransferStatusNotCopied {
		s := formatTime(time.Now().UTC())
		finishedAt = &s
	}
	_, err := r.db.Exec(
		`UPDATE transfers SET status=?, error_msg=?, duration_ms=?, finished_at=? WHERE id=?`,
		status, errMsg, durationMs, finishedAt, id,
	)
	return err
}

// MarkPendingNotCopied marks all pending (and in_progress) transfers for a run
// as not_copied. Call this after a run ends to ensure no transfers are left in
// a transient state.
func (r *TransferRepository) MarkPendingNotCopied(runID string) error {
	finished := formatTime(time.Now().UTC())
	_, err := r.db.Exec(
		`UPDATE transfers SET status=?, finished_at=? WHERE run_id=? AND status IN (?, ?)`,
		TransferStatusNotCopied, finished, runID, TransferStatusPending, TransferStatusInProgress,
	)
	return err
}

func (r *TransferRepository) ListByRun(runID string) ([]*Transfer, error) {
	rows, err := r.db.Query(
		`SELECT id, run_id, remote_path, local_path, size_bytes, bytes_xferred, duration_ms, status, error_msg, started_at, finished_at, previous_commit_hash, current_commit_hash
		 FROM transfers WHERE run_id = ? ORDER BY remote_path`, runID,
	)
	if err != nil {
		return nil, fmt.Errorf("list transfers: %w", err)
	}
	defer rows.Close()

	var transfers []*Transfer
	for rows.Next() {
		t, err := scanTransfer(rows)
		if err != nil {
			return nil, err
		}
		transfers = append(transfers, t)
	}
	return transfers, rows.Err()
}

func scanTransfer(s scanner) (*Transfer, error) {
	var t Transfer
	var durationMs sql.NullInt64
	var errMsg, startedAt, finishedAt, prevHash, currHash sql.NullString

	err := s.Scan(
		&t.ID, &t.RunID, &t.RemotePath, &t.LocalPath,
		&t.SizeBytes, &t.BytesXferred, &durationMs,
		&t.Status, &errMsg, &startedAt, &finishedAt,
		&prevHash, &currHash,
	)
	if err != nil {
		return nil, fmt.Errorf("scan transfer: %w", err)
	}

	if durationMs.Valid {
		t.DurationMs = &durationMs.Int64
	}
	if errMsg.Valid {
		t.ErrorMsg = &errMsg.String
	}
	if startedAt.Valid {
		ts, _ := time.Parse(time.RFC3339, startedAt.String)
		t.StartedAt = &ts
	}
	if finishedAt.Valid {
		ts, _ := time.Parse(time.RFC3339, finishedAt.String)
		t.FinishedAt = &ts
	}
	if prevHash.Valid {
		t.PreviousCommitHash = &prevHash.String
	}
	if currHash.Valid {
		t.CurrentCommitHash = &currHash.String
	}
	return &t, nil
}
