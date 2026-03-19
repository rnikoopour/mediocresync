package db

import (
	"database/sql"
	"fmt"
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

func (r *FileStateRepository) DeleteByJob(jobID string) error {
	_, err := r.db.Exec(`DELETE FROM file_state WHERE job_id=?`, jobID)
	return err
}
