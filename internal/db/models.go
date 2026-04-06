package db

import "time"

// Run status values.
const (
	RunStatusRunning       = "running"
	RunStatusCompleted     = "completed"
	RunStatusNothingToSync = "nothing_to_sync"
	RunStatusFailed        = "failed"
	RunStatusPartial       = "partial"
	RunStatusCanceled      = "canceled"
	RunStatusServerStopped = "server_stopped"
)

// Transfer status values.
const (
	TransferStatusPending    = "pending"
	TransferStatusInProgress = "in_progress"
	TransferStatusDone       = "done"
	TransferStatusSkipped    = "skipped"
	TransferStatusFailed     = "failed"
	TransferStatusNotCopied  = "not_copied"
)

// Source type values.
const (
	SourceTypeFTPES = "ftpes"
	SourceTypeGit   = "git"
)

// Auth type values for Git sources.
const (
	AuthTypeNone   = "none"
	AuthTypeToken  = "token"
	AuthTypeSSHKey = "ssh_key"
)

type Source struct {
	ID             string
	Name           string
	Type           string // "ftpes" | "git"
	Host           string // FTPES only; empty for Git
	Port           int    // FTPES only; 0 for Git
	Username       string // FTPES only; empty for Git
	Password       []byte // AES-256-GCM encrypted; nil for Git
	SkipTLSVerify  bool   // FTPES only
	EnableEPSV     bool   // FTPES only
	AuthType       string // Git only: "none" | "token" | "ssh_key"; empty for FTPES
	AuthCredential []byte // AES-256-GCM encrypted; Git only; nil for FTPES
	CreatedAt      time.Time
	UpdatedAt      time.Time
}

type GitRepo struct {
	ID     string
	JobID  string
	URL    string
	Branch string
}

type SyncJob struct {
	ID                string
	Name              string
	SourceID          string
	RemotePath        string
	LocalDest         string
	IntervalValue     int
	IntervalUnit      string // minutes | hours | days
	Concurrency       int    // number of files to download concurrently (default 1)
	RetryAttempts     int    // number of attempts per file (1 = no retry)
	RetryDelaySeconds int    // seconds to wait between attempts
	Enabled           bool
	IncludePathFilters []string // subdirectory names; file must be under at least one (if non-empty)
	IncludeNameFilters []string // basename glob patterns; file basename must match at least one (if non-empty)
	ExcludePathFilters []string // file excluded if under any of these
	ExcludeNameFilters []string // file excluded if basename matches any of these
	RunRetentionDays  int      // days to keep run history; 0 = keep forever
	CreatedAt         time.Time
	UpdatedAt         time.Time
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
	ID                 string
	RunID              string
	RemotePath         string
	LocalPath          string
	SizeBytes          int64
	BytesXferred       int64
	DurationMs         *int64
	Status             string // pending | in_progress | done | skipped | failed
	ErrorMsg           *string
	StartedAt          *time.Time
	FinishedAt         *time.Time
	PreviousCommitHash *string // git only: hash last synced before this run
	CurrentCommitHash  *string // git only: hash synced during this run
}

type SyncState struct {
	JobID       string
	RemotePath  string
	SizeBytes   int64
	MTime       *time.Time // nil for Git repos
	ContentHash *string    // git commit hash; nil for FTPES
	CopiedAt    time.Time
}
