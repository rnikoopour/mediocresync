package db

import "time"

type Connection struct {
	ID            string
	Name          string
	Host          string
	Port          int
	Username      string
	Password      []byte // AES-256-GCM encrypted; decrypt only when dialing FTPES
	SkipTLSVerify bool
	CreatedAt     time.Time
	UpdatedAt     time.Time
}

type SyncJob struct {
	ID             string
	Name           string
	ConnectionID   string
	RemotePath     string
	LocalDest      string
	IntervalValue  int
	IntervalUnit   string // minutes | hours | days
	Concurrency    int    // number of files to download concurrently (default 1)
	Enabled   bool
	Filters   []string // "path: <subdir>" or "name: <glob>" entries; empty = include all
	CreatedAt time.Time
	UpdatedAt      time.Time
}

type Run struct {
	ID           string
	JobID        string
	Status       string // running | completed | failed
	StartedAt    time.Time
	FinishedAt   *time.Time
	TotalFiles   int
	CopiedFiles  int
	SkippedFiles int
	FailedFiles  int
}

type Transfer struct {
	ID           string
	RunID        string
	RemotePath   string
	LocalPath    string
	SizeBytes    int64
	BytesXferred int64
	DurationMs   *int64
	Status       string // pending | in_progress | done | skipped | failed
	ErrorMsg     *string
	StartedAt    *time.Time
	FinishedAt   *time.Time
}

type FileState struct {
	JobID      string
	RemotePath string
	SizeBytes  int64
	MTime      time.Time
	CopiedAt   time.Time
}
