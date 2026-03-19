package db

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
)

type JobRepository struct {
	db *sql.DB
}

func NewJobRepository(db *sql.DB) *JobRepository {
	return &JobRepository{db: db}
}

func (r *JobRepository) Create(j *SyncJob) error {
	j.ID = uuid.New().String()
	now := time.Now().UTC()
	j.CreatedAt = now
	j.UpdatedAt = now

	inc, exc := marshalFilters(j.IncludeFilters), marshalFilters(j.ExcludeFilters)
	_, err := r.db.Exec(
		`INSERT INTO sync_jobs (id, name, connection_id, remote_path, local_dest, interval_value, interval_unit, concurrency, enabled, include_filters, exclude_filters, created_at, updated_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		j.ID, j.Name, j.ConnectionID, j.RemotePath, j.LocalDest,
		j.IntervalValue, j.IntervalUnit, j.Concurrency, boolToInt(j.Enabled),
		inc, exc, formatTime(j.CreatedAt), formatTime(j.UpdatedAt),
	)
	if err != nil {
		return fmt.Errorf("insert sync job: %w", err)
	}
	return nil
}

const jobColumns = `id, name, connection_id, remote_path, local_dest, interval_value, interval_unit, concurrency, enabled, include_filters, exclude_filters, created_at, updated_at`

func (r *JobRepository) List() ([]*SyncJob, error) {
	return r.query(`SELECT ` + jobColumns + ` FROM sync_jobs ORDER BY name`)
}

func (r *JobRepository) ListEnabled() ([]*SyncJob, error) {
	return r.query(`SELECT ` + jobColumns + ` FROM sync_jobs WHERE enabled = 1 ORDER BY name`)
}

func (r *JobRepository) Get(id string) (*SyncJob, error) {
	row := r.db.QueryRow(`SELECT `+jobColumns+` FROM sync_jobs WHERE id = ?`, id)
	j, err := scanJob(row)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	return j, err
}

func (r *JobRepository) Update(j *SyncJob) error {
	j.UpdatedAt = time.Now().UTC()
	inc, exc := marshalFilters(j.IncludeFilters), marshalFilters(j.ExcludeFilters)
	res, err := r.db.Exec(
		`UPDATE sync_jobs SET name=?, connection_id=?, remote_path=?, local_dest=?, interval_value=?, interval_unit=?, concurrency=?, enabled=?, include_filters=?, exclude_filters=?, updated_at=?
		 WHERE id=?`,
		j.Name, j.ConnectionID, j.RemotePath, j.LocalDest,
		j.IntervalValue, j.IntervalUnit, j.Concurrency, boolToInt(j.Enabled),
		inc, exc, formatTime(j.UpdatedAt), j.ID,
	)
	if err != nil {
		return fmt.Errorf("update sync job: %w", err)
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return fmt.Errorf("sync job %s not found", j.ID)
	}
	return nil
}

func (r *JobRepository) Delete(id string) error {
	_, err := r.db.Exec(`DELETE FROM sync_jobs WHERE id = ?`, id)
	return err
}

func marshalFilters(f []string) string {
	if len(f) == 0 {
		return "[]"
	}
	b, _ := json.Marshal(f)
	return string(b)
}

func unmarshalFilters(s string) []string {
	var f []string
	_ = json.Unmarshal([]byte(s), &f)
	if f == nil {
		return []string{}
	}
	return f
}

func (r *JobRepository) query(q string) ([]*SyncJob, error) {
	rows, err := r.db.Query(q)
	if err != nil {
		return nil, fmt.Errorf("query jobs: %w", err)
	}
	defer rows.Close()

	var jobs []*SyncJob
	for rows.Next() {
		j, err := scanJob(rows)
		if err != nil {
			return nil, err
		}
		jobs = append(jobs, j)
	}
	return jobs, rows.Err()
}

func scanJob(s scanner) (*SyncJob, error) {
	var j SyncJob
	var enabled int
	var includeFilters, excludeFilters string
	var createdAt, updatedAt string

	err := s.Scan(
		&j.ID, &j.Name, &j.ConnectionID, &j.RemotePath, &j.LocalDest,
		&j.IntervalValue, &j.IntervalUnit, &j.Concurrency, &enabled,
		&includeFilters, &excludeFilters, &createdAt, &updatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("scan job: %w", err)
	}

	j.Enabled = enabled == 1
	j.IncludeFilters = unmarshalFilters(includeFilters)
	j.ExcludeFilters = unmarshalFilters(excludeFilters)
	j.CreatedAt, _ = time.Parse(time.RFC3339, createdAt)
	j.UpdatedAt, _ = time.Parse(time.RFC3339, updatedAt)
	return &j, nil
}
