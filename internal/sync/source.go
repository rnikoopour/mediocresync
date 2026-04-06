package sync

import (
	"context"
	"time"

	"github.com/rnikoopour/mediocresync/internal/db"
)

// PlanFile describes what would happen to a single remote file if the job ran.
type PlanFile struct {
	RemotePath         string    `json:"remote_path"`
	LocalPath          string    `json:"local_path"`
	SizeBytes          int64     `json:"size_bytes"`
	MTime              time.Time `json:"mtime"`
	Action             string    `json:"action"`                         // "copy" | "skip" | "error"
	CommitHash         string    `json:"commit_hash,omitempty"`          // git only: current remote hash
	PreviousCommitHash string    `json:"previous_commit_hash,omitempty"` // git only: last synced hash
	Error              string    `json:"error,omitempty"`                // git only: per-repo plan error
}

// PlanOutput is the full output of a dry-run plan.
type PlanOutput struct {
	Files  []PlanFile `json:"files"`
	ToCopy int        `json:"to_copy"`
	ToSkip int        `json:"to_skip"`
}

// PlanInput carries inputs to Source.Plan.
// Two non-ctx fields → struct.
type PlanInput struct {
	// Progress is called after each file or directory is discovered during the
	// remote walk. May be nil.
	Progress func(files, dirs int)
	// LookupState returns the last-synced state for a remote path. The engine
	// supplies a closure scoped to the current job ID.
	LookupState func(remotePath string) (*db.SyncState, error)
}

// TransferEventKind identifies the type of transfer event emitted by Source.Sync.
type TransferEventKind string

const (
	// TransferEventStarted signals that a transfer has begun (git: in_progress).
	TransferEventStarted TransferEventKind = "started"
	// TransferEventProgress carries byte-level progress (FTPES only).
	TransferEventProgress TransferEventKind = "progress"
	// TransferEventRetrying signals that a failed attempt is being retried.
	// BytesXferred carries the safely staged byte count so the UI can preserve
	// the progress floor instead of resetting to 0.
	TransferEventRetrying TransferEventKind = "retrying"
	// TransferEventDone signals a successful transfer completion.
	TransferEventDone TransferEventKind = "done"
	// TransferEventFailed signals a transfer failure.
	TransferEventFailed TransferEventKind = "failed"
)

// TransferEvent is emitted by Source.Sync via the SyncInput.OnEvent callback.
// The engine performs all db writes and SSE publishes in response to these events;
// sources contain no db fields.
type TransferEvent struct {
	Kind         TransferEventKind
	RemotePath   string
	Error        string  // Kind == failed only
	BytesXferred int64   // Kind == progress/done
	SizeBytes    int64
	SpeedBPS     float64
	DurationMs   *int64 // Kind == done only

	// Done-only post-completion fields. The source sets these explicitly;
	// the engine acts on non-nil values without inferring the source type.
	SyncState  *db.SyncState // non-nil → engine calls syncState.Upsert
	CommitHash *string       // non-nil → engine calls UpdateCurrentCommitHash
}

// SyncInput carries inputs to Source.Sync.
// Two non-ctx fields → struct.
type SyncInput struct {
	Plan    *PlanOutput
	OnEvent func(TransferEvent)
}

// Source is implemented by FTPESSource and GitSource. The engine calls Plan to
// determine what work needs to be done, then Sync to execute it.
//
// Sources have no db fields. All persistence (transfer records, run counts, sync
// state) is handled by the engine via the OnEvent callback.
//
// Sync always returns nil; transfer-level errors are communicated through
// OnEvent(TransferEventFailed). Sync returns a non-nil error only for hard
// setup failures (e.g. unable to create staging directory) that prevent any
// transfers from starting.
type Source interface {
	Plan(ctx context.Context, in PlanInput) (*PlanOutput, error)
	Sync(ctx context.Context, in SyncInput) error
}
