package db

import (
	"database/sql"
	"fmt"

	"github.com/google/uuid"
)

type GitRepoRepository struct {
	db *sql.DB
}

func NewGitRepoRepository(db *sql.DB) *GitRepoRepository {
	return &GitRepoRepository{db: db}
}

func (r *GitRepoRepository) ListByJob(jobID string) ([]*GitRepo, error) {
	rows, err := r.db.Query(
		`SELECT id, job_id, url, branch FROM git_repos WHERE job_id = ? ORDER BY url`,
		jobID,
	)
	if err != nil {
		return nil, fmt.Errorf("list git repos: %w", err)
	}
	defer rows.Close()

	var repos []*GitRepo
	for rows.Next() {
		var g GitRepo
		if err := rows.Scan(&g.ID, &g.JobID, &g.URL, &g.Branch); err != nil {
			return nil, fmt.Errorf("scan git repo: %w", err)
		}
		repos = append(repos, &g)
	}
	return repos, rows.Err()
}

// ReplaceForJob deletes all git_repos for the job and inserts the provided list atomically.
func (r *GitRepoRepository) ReplaceForJob(jobID string, repos []*GitRepo) error {
	tx, err := r.db.Begin()
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback() //nolint:errcheck

	if _, err := tx.Exec(`DELETE FROM git_repos WHERE job_id = ?`, jobID); err != nil {
		return fmt.Errorf("delete git repos: %w", err)
	}
	for _, g := range repos {
		if g.ID == "" {
			g.ID = uuid.New().String()
		}
		g.JobID = jobID
		if g.Branch == "" {
			g.Branch = "main"
		}
		if _, err := tx.Exec(
			`INSERT INTO git_repos (id, job_id, url, branch) VALUES (?, ?, ?, ?)`,
			g.ID, g.JobID, g.URL, g.Branch,
		); err != nil {
			return fmt.Errorf("insert git repo: %w", err)
		}
	}
	return tx.Commit()
}
