package db

import (
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
)

type RunRepository struct {
	db *sql.DB
}

func NewRunRepository(db *sql.DB) *RunRepository {
	return &RunRepository{db: db}
}

func (r *RunRepository) Create(run *Run) error {
	run.ID = uuid.New().String()
	run.StartedAt = time.Now().UTC()

	_, err := r.db.Exec(
		`INSERT INTO runs (id, job_id, status, started_at, total_files, copied_files, skipped_files, failed_files)
		 VALUES (?, ?, ?, ?, 0, 0, 0, 0)`,
		run.ID, run.JobID, run.Status, formatTime(run.StartedAt),
	)
	if err != nil {
		return fmt.Errorf("insert run: %w", err)
	}
	return nil
}

func (r *RunRepository) UpdateTotalSize(id string, bytes int64) error {
	_, err := r.db.Exec(`UPDATE runs SET total_size_bytes=? WHERE id=?`, bytes, id)
	return err
}

func (r *RunRepository) Get(id string) (*Run, error) {
	row := r.db.QueryRow(
		`SELECT id, job_id, status, started_at, finished_at, total_files, copied_files, skipped_files, failed_files, total_size_bytes
		 FROM runs WHERE id = ?`, id,
	)
	run, err := scanRun(row)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	return run, err
}

func (r *RunRepository) ListByJob(jobID string) ([]*Run, error) {
	rows, err := r.db.Query(
		`SELECT id, job_id, status, started_at, finished_at, total_files, copied_files, skipped_files, failed_files, total_size_bytes
		 FROM runs WHERE job_id = ? ORDER BY started_at DESC`, jobID,
	)
	if err != nil {
		return nil, fmt.Errorf("list runs: %w", err)
	}
	defer rows.Close()

	var runs []*Run
	for rows.Next() {
		run, err := scanRun(rows)
		if err != nil {
			return nil, err
		}
		runs = append(runs, run)
	}
	return runs, rows.Err()
}

func (r *RunRepository) UpdateStatus(id, status string) error {
	var finishedAt *string
	if status == "completed" || status == "failed" || status == "canceled" || status == "server_stopped" {
		s := formatTime(time.Now().UTC())
		finishedAt = &s
	}
	_, err := r.db.Exec(
		`UPDATE runs SET status=?, finished_at=? WHERE id=?`,
		status, finishedAt, id,
	)
	return err
}

// CancelStaleRuns marks any runs still in "running" state as "server_stopped".
// Call this on startup to clean up runs that were interrupted by an unclean shutdown.
func (r *RunRepository) CancelStaleRuns() error {
	finished := formatTime(time.Now().UTC())
	_, err := r.db.Exec(
		`UPDATE runs SET status='server_stopped', finished_at=? WHERE status='running'`,
		finished,
	)
	return err
}

func (r *RunRepository) UpdateCounts(id string, total, copied, skipped, failed int) error {
	_, err := r.db.Exec(
		`UPDATE runs SET total_files=?, copied_files=?, skipped_files=?, failed_files=? WHERE id=?`,
		total, copied, skipped, failed, id,
	)
	return err
}

func scanRun(s scanner) (*Run, error) {
	var run Run
	var startedAt string
	var finishedAt *string

	err := s.Scan(
		&run.ID, &run.JobID, &run.Status, &startedAt, &finishedAt,
		&run.TotalFiles, &run.CopiedFiles, &run.SkippedFiles, &run.FailedFiles, &run.TotalSizeBytes,
	)
	if err != nil {
		return nil, fmt.Errorf("scan run: %w", err)
	}

	run.StartedAt, _ = time.Parse(time.RFC3339, startedAt)
	if finishedAt != nil {
		t, _ := time.Parse(time.RFC3339, *finishedAt)
		run.FinishedAt = &t
	}
	return &run, nil
}
