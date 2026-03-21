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
	EnableEPSV    bool
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
	Concurrency      int    // number of files to download concurrently (default 1)
	RetryAttempts    int    // number of attempts per file (1 = no retry)
	RetryDelaySeconds int   // seconds to wait between attempts
	Enabled          bool
	IncludePathFilters []string // subdirectory names; file must be under at least one (if non-empty)
	IncludeNameFilters []string // basename glob patterns; file basename must match at least one (if non-empty)
	ExcludePathFilters []string // file excluded if under any of these
	ExcludeNameFilters []string // file excluded if basename matches any of these
	RunRetentionDays int      // days to keep run history; 0 = keep forever
	CreatedAt      time.Time
	UpdatedAt      time.Time
}

type Run struct {
	ID             string
	JobID          string
	Status         string // running | completed | nothing_to_sync | failed | canceled | server_stopped
	StartedAt      time.Time
	FinishedAt     *time.Time
	TotalFiles     int
	CopiedFiles    int
	SkippedFiles   int
	FailedFiles    int
	TotalSizeBytes int64
	ErrorMsg       *string
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
