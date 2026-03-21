package db

import (
	"database/sql"
	"fmt"
	"strings"
	"time"
)

type FileStateRepository struct {
	db *sql.DB
}

func NewFileStateRepository(db *sql.DB) *FileStateRepository {
	return &FileStateRepository{db: db}
}

func (r *FileStateRepository) Upsert(s *FileState) error {
	_, err := r.db.Exec(
		`INSERT INTO file_state (job_id, remote_path, size_bytes, mtime, copied_at)
		 VALUES (?, ?, ?, ?, ?)
		 ON CONFLICT(job_id, remote_path) DO UPDATE SET
		   size_bytes=excluded.size_bytes,
		   mtime=excluded.mtime,
		   copied_at=excluded.copied_at`,
		s.JobID, s.RemotePath, s.SizeBytes, formatTime(s.MTime), formatTime(s.CopiedAt),
	)
	if err != nil {
		return fmt.Errorf("upsert file state: %w", err)
	}
	return nil
}

func (r *FileStateRepository) Get(jobID, remotePath string) (*FileState, error) {
	row := r.db.QueryRow(
		`SELECT job_id, remote_path, size_bytes, mtime, copied_at FROM file_state WHERE job_id=? AND remote_path=?`,
		jobID, remotePath,
	)

	var s FileState
	var mtime, copiedAt string

	err := row.Scan(&s.JobID, &s.RemotePath, &s.SizeBytes, &mtime, &copiedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get file state: %w", err)
	}

	s.MTime, _ = time.Parse(time.RFC3339, mtime)
	s.CopiedAt, _ = time.Parse(time.RFC3339, copiedAt)
	return &s, nil
}

func (r *FileStateRepository) Delete(jobID, remotePath string) error {
	_, err := r.db.Exec(`DELETE FROM file_state WHERE job_id=? AND remote_path=?`, jobID, remotePath)
	return err
}

func (r *FileStateRepository) DeleteByJob(jobID string) error {
	_, err := r.db.Exec(`DELETE FROM file_state WHERE job_id=?`, jobID)
	return err
}

// PruneStale removes file_state entries for jobID whose remote_path is not in
// knownPaths. It fetches the current paths for the job, computes the diff in
// Go, and deletes in batches of 500 to stay well under SQLite's bind-parameter limit.
func (r *FileStateRepository) PruneStale(jobID string, knownPaths []string) (int, error) {
	known := make(map[string]struct{}, len(knownPaths))
	for _, p := range knownPaths {
		known[p] = struct{}{}
	}

	rows, err := r.db.Query(`SELECT remote_path FROM file_state WHERE job_id = ?`, jobID)
	if err != nil {
		return 0, fmt.Errorf("prune file state: %w", err)
	}
	defer rows.Close()

	var stale []string
	for rows.Next() {
		var path string
		if err := rows.Scan(&path); err != nil {
			return 0, fmt.Errorf("prune file state: %w", err)
		}
		if _, ok := known[path]; !ok {
			stale = append(stale, path)
		}
	}
	if err := rows.Err(); err != nil {
		return 0, fmt.Errorf("prune file state: %w", err)
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
			`DELETE FROM file_state WHERE job_id = ? AND remote_path IN (`+placeholders+`)`,
			args...,
		); err != nil {
			return 0, fmt.Errorf("prune file state: %w", err)
		}
	}
	return len(stale), nil
}
