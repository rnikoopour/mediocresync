package db

import (
	"database/sql"
	"fmt"
	"strings"
	"time"
)

type SyncStateRepository struct {
	db *sql.DB
}

func NewSyncStateRepository(db *sql.DB) *SyncStateRepository {
	return &SyncStateRepository{db: db}
}

func (r *SyncStateRepository) Upsert(s *SyncState) error {
	var mtime sql.NullString
	if s.MTime != nil {
		mtime = sql.NullString{String: formatTime(*s.MTime), Valid: true}
	}
	var contentHash sql.NullString
	if s.ContentHash != nil {
		contentHash = sql.NullString{String: *s.ContentHash, Valid: true}
	}

	_, err := r.db.Exec(
		`INSERT INTO sync_state (job_id, remote_path, size_bytes, mtime, content_hash, copied_at)
		 VALUES (?, ?, ?, ?, ?, ?)
		 ON CONFLICT(job_id, remote_path) DO UPDATE SET
		   size_bytes=excluded.size_bytes,
		   mtime=excluded.mtime,
		   content_hash=excluded.content_hash,
		   copied_at=excluded.copied_at`,
		s.JobID, s.RemotePath, s.SizeBytes, mtime, contentHash, formatTime(s.CopiedAt),
	)
	if err != nil {
		return fmt.Errorf("upsert sync state: %w", err)
	}
	return nil
}

func (r *SyncStateRepository) Get(jobID, remotePath string) (*SyncState, error) {
	row := r.db.QueryRow(
		`SELECT job_id, remote_path, size_bytes, mtime, content_hash, copied_at FROM sync_state WHERE job_id=? AND remote_path=?`,
		jobID, remotePath,
	)

	var s SyncState
	var mtime, contentHash sql.NullString
	var copiedAt string

	err := row.Scan(&s.JobID, &s.RemotePath, &s.SizeBytes, &mtime, &contentHash, &copiedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get sync state: %w", err)
	}

	if mtime.Valid {
		t, _ := time.Parse(time.RFC3339, mtime.String)
		s.MTime = &t
	}
	if contentHash.Valid {
		s.ContentHash = &contentHash.String
	}
	s.CopiedAt, _ = time.Parse(time.RFC3339, copiedAt)
	return &s, nil
}

func (r *SyncStateRepository) Delete(jobID, remotePath string) error {
	_, err := r.db.Exec(`DELETE FROM sync_state WHERE job_id=? AND remote_path=?`, jobID, remotePath)
	return err
}

func (r *SyncStateRepository) DeleteByJob(jobID string) error {
	_, err := r.db.Exec(`DELETE FROM sync_state WHERE job_id=?`, jobID)
	return err
}

// PruneStale removes sync_state entries for jobID whose remote_path is not in
// knownPaths. It fetches the current paths for the job, computes the diff in
// Go, and deletes in batches of 500 to stay well under SQLite's bind-parameter limit.
func (r *SyncStateRepository) PruneStale(jobID string, knownPaths []string) (int, error) {
	known := make(map[string]struct{}, len(knownPaths))
	for _, p := range knownPaths {
		known[p] = struct{}{}
	}

	rows, err := r.db.Query(`SELECT remote_path FROM sync_state WHERE job_id = ?`, jobID)
	if err != nil {
		return 0, fmt.Errorf("prune sync state: %w", err)
	}
	defer rows.Close()

	var stale []string
	for rows.Next() {
		var path string
		if err := rows.Scan(&path); err != nil {
			return 0, fmt.Errorf("prune sync state: %w", err)
		}
		if _, ok := known[path]; !ok {
			stale = append(stale, path)
		}
	}
	if err := rows.Err(); err != nil {
		return 0, fmt.Errorf("prune sync state: %w", err)
	}

	const batchSize = 500
	for i := 0; i < len(stale); i += batchSize {
		batch := stale[i:min(i+batchSize, len(stale))]
		placeholders := strings.Repeat("?,", len(batch))
		placeholders = placeholders[:len(placeholders)-1]
		args := make([]any, 1+len(batch))
		args[0] = jobID
		for j, p := range batch {
			args[j+1] = p
		}
		if _, err := r.db.Exec(
			`DELETE FROM sync_state WHERE job_id = ? AND remote_path IN (`+placeholders+`)`,
			args...,
		); err != nil {
			return 0, fmt.Errorf("prune sync state: %w", err)
		}
	}
	return len(stale), nil
}
