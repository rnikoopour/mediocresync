package sync

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/rnikoopour/mediocresync/internal/db"
	"github.com/rnikoopour/mediocresync/internal/sse"
)

// ErrJobAlreadyRunning is returned by RunJob when a run for the job is already active.
var ErrJobAlreadyRunning = fmt.Errorf("job is already running")

// failedTransferError is returned by runWithPlan when every file to be
// transferred failed (none succeeded). The run is marked "failed".
type failedTransferError struct{ failed int }

func (e failedTransferError) Error() string {
	return fmt.Sprintf("%d file(s) failed to transfer", e.failed)
}

// partialTransferError is returned by runWithPlan when at least one file failed
// but at least one other was successfully transferred. Distinct from a total
// failure so the run can be marked "partial" rather than "failed".
type partialTransferError struct {
	completed int
	failed    int
}

func (e partialTransferError) Error() string {
	return fmt.Sprintf("%d file(s) succeeded, %d file(s) failed to transfer", e.completed, e.failed)
}

const stallTimeout = 30 * time.Second

var errTransferStalled = errors.New("transfer stalled: no data received for 30s")

// ErrPlanAlreadyRunning is returned by StartPlan when a plan for the job is already active.
var ErrPlanAlreadyRunning = fmt.Errorf("plan is already running")

// PlanEvent is a progress or terminal event broadcast to plan SSE subscribers.
type PlanEvent struct {
	Files     int         `json:"files"`
	Folders   int         `json:"folders"`
	Done      bool        `json:"done"`
	Dismissed bool        `json:"dismissed"`
	Error     string      `json:"error"`
	Result    *PlanOutput `json:"result,omitempty"`
}

type Engine struct {
	sources   *db.SourceRepository
	gitRepos  *db.GitRepoRepository
	jobs      *db.JobRepository
	runs      *db.RunRepository
	transfers *db.TransferRepository
	syncState *db.SyncStateRepository
	encKey    []byte
	broker    *sse.Broker
	appCtx    context.Context // cancelled on server shutdown

	mu            sync.Mutex
	active        map[string]bool            // jobID → running
	activeRunIDs  map[string]string          // jobID → current run ID
	cancelFuncs   map[string]context.CancelFunc
	storedPlans   map[string]*PlanOutput
	storedPlansMu sync.Mutex
	runWG         sync.WaitGroup // tracks all in-flight runWithPlan calls

	planMu       sync.Mutex
	planActive   map[string]bool
	planProgress map[string]PlanEvent // latest progress event per active plan
	planSubs     map[string][]chan PlanEvent

	// sourceFactory overrides newSource when set. Used in tests to inject a
	// mock source without wiring up real FTP/git credentials.
	sourceFactory func(job *db.SyncJob, src *db.Source) (Source, error)
}

func NewEngine(
	sources *db.SourceRepository,
	gitRepos *db.GitRepoRepository,
	jobs *db.JobRepository,
	runs *db.RunRepository,
	transfers *db.TransferRepository,
	syncState *db.SyncStateRepository,
	encKey []byte,
	broker *sse.Broker,
	appCtx context.Context,
) *Engine {
	return &Engine{
		sources:   sources,
		gitRepos:  gitRepos,
		jobs:      jobs,
		runs:      runs,
		transfers: transfers,
		syncState: syncState,
		encKey:    encKey,
		broker:    broker,
		appCtx:    appCtx,
		active:       make(map[string]bool),
		activeRunIDs: make(map[string]string),
		cancelFuncs:  make(map[string]context.CancelFunc),
		storedPlans: make(map[string]*PlanOutput),
		planActive:   make(map[string]bool),
		planProgress: make(map[string]PlanEvent),
		planSubs:     make(map[string][]chan PlanEvent),
	}
}

// newSource constructs the appropriate Source implementation for the given job
// and source config. It accesses engine fields (encKey, gitRepos, appCtx)
// directly so callers don't need to pass them.
func (e *Engine) newSource(job *db.SyncJob, src *db.Source) (Source, error) {
	switch src.Type {
	case db.SourceTypeFTPES:
		return newFTPESSource(job, src, e.encKey, e.appCtx), nil
	case db.SourceTypeGit:
		repos, err := e.gitRepos.ListByJob(job.ID)
		if err != nil {
			return nil, fmt.Errorf("list git repos: %w", err)
		}
		return newGitSource(job, src, repos, e.encKey, e.appCtx), nil
	default:
		return nil, fmt.Errorf("unknown source type %q", src.Type)
	}
}

// makeSource returns the source for a job. If sourceFactory is set (e.g. in
// tests), it is used instead of newSource.
func (e *Engine) makeSource(job *db.SyncJob, src *db.Source) (Source, error) {
	if e.sourceFactory != nil {
		return e.sourceFactory(job, src)
	}
	return e.newSource(job, src)
}

// PlanJob connects to the remote, walks the tree, and returns which files
// would be copied or skipped — without downloading anything.
func (e *Engine) PlanJob(ctx context.Context, jobID string) (*PlanOutput, error) {
	return e.PlanJobStream(ctx, jobID, nil)
}

// PlanJobStream is like PlanJob but calls progress(files, dirs) after each
// file or directory is discovered during the remote walk.
func (e *Engine) PlanJobStream(ctx context.Context, jobID string, progress func(files, dirs int)) (*PlanOutput, error) {
	job, err := e.jobs.Get(jobID)
	if err != nil || job == nil {
		return nil, fmt.Errorf("load job %s: %w", jobID, err)
	}

	src, err := e.sources.Get(job.SourceID)
	if err != nil || src == nil {
		return nil, fmt.Errorf("load source %s: %w", job.SourceID, err)
	}

	source, err := e.makeSource(job, src)
	if err != nil {
		return nil, fmt.Errorf("build source: %w", err)
	}

	result, err := source.Plan(ctx, PlanInput{
		Progress: progress,
		LookupState: func(remotePath string) (*db.SyncState, error) {
			return e.syncState.Get(jobID, remotePath)
		},
	})
	if err != nil {
		return nil, err
	}

	e.storedPlansMu.Lock()
	e.storedPlans[jobID] = result
	e.storedPlansMu.Unlock()

	return result, nil
}

// IsRunning reports whether a run for the given job is currently active.
func (e *Engine) IsRunning(jobID string) bool {
	e.mu.Lock()
	defer e.mu.Unlock()
	return e.active[jobID]
}

// IsPlanning reports whether a plan scan for the given job is currently active.
func (e *Engine) IsPlanning(jobID string) bool {
	e.planMu.Lock()
	defer e.planMu.Unlock()
	return e.planActive[jobID]
}

// RunJob executes a full sync for the given job using a previously stored plan.
// Returns an error if no plan has been computed via PlanJob/PlanJobStream first.
// Returns ErrJobAlreadyRunning if a run for this job is already in progress.
func (e *Engine) RunJob(jobID string) error {
	e.storedPlansMu.Lock()
	plan := e.storedPlans[jobID]
	delete(e.storedPlans, jobID)
	e.storedPlansMu.Unlock()

	if plan == nil {
		return fmt.Errorf("no plan available: run Plan before running")
	}
	return e.runWithPlan(e.appCtx, jobID, plan)
}

// PlanThenRun plans and immediately runs the given job without user interaction.
// Used by the scheduler for automated runs.
func (e *Engine) PlanThenRun(ctx context.Context, jobID string) error {
	// Signal job-level clients (e.g. jobs list page) that planning has started.
	e.broker.Publish(jobID, sse.Event{Status: "planning"})

	// Mark the plan as active and broadcast an initial event so plan subscribers
	// (e.g. job detail page) immediately see the "running" state.
	e.planMu.Lock()
	e.planActive[jobID] = true
	e.planMu.Unlock()
	e.broadcastPlanEvent(jobID, PlanEvent{})

	plan, err := e.PlanJobStream(ctx, jobID, func(files, dirs int) {
		e.broadcastPlanEvent(jobID, PlanEvent{Files: files, Folders: dirs})
	})

	e.planMu.Lock()
	delete(e.planActive, jobID)
	e.planMu.Unlock()

	if err != nil {
		e.broadcastPlanEvent(jobID, PlanEvent{Error: err.Error()})
		// Record a failed run so the scheduler sees a recent attempt and does
		// not retry every minute while the error persists.
		run := &db.Run{JobID: jobID, Status: db.RunStatusRunning}
		if createErr := e.runs.Create(run); createErr != nil {
			slog.Error("plan failed: could not record failed run", "job_id", jobID, "err", createErr)
		} else {
			msg := err.Error()
			if updateErr := e.runs.UpdateStatus(run.ID, db.RunStatusFailed, &msg); updateErr != nil {
				slog.Error("plan failed: could not update run status", "job_id", jobID, "run_id", run.ID, "err", updateErr)
			}
			e.broker.Publish(jobID, sse.Event{RunID: run.ID, Status: "failed"})
		}
		return fmt.Errorf("plan: %w", err)
	}

	// Broadcast the completed plan so job detail page can display it briefly.
	e.broadcastPlanEvent(jobID, PlanEvent{Done: true, Result: plan})

	// Discard stored copy — we use the result directly rather than requiring
	// a separate RunJob call. Also dismiss the plan from connected clients
	// so it clears from the job detail page when the run begins.
	e.storedPlansMu.Lock()
	delete(e.storedPlans, jobID)
	e.storedPlansMu.Unlock()
	e.broadcastPlanEvent(jobID, PlanEvent{Dismissed: true})

	return e.runWithPlan(ctx, jobID, plan)
}

// StartPlan starts a plan scan in the background and broadcasts progress to
// all current and future SSE subscribers for this job.
// Returns ErrPlanAlreadyRunning if a plan for this job is already in progress.
func (e *Engine) StartPlan(jobID string) error {
	e.planMu.Lock()
	if e.planActive[jobID] {
		e.planMu.Unlock()
		return ErrPlanAlreadyRunning
	}
	e.planActive[jobID] = true
	e.planMu.Unlock()

	// Immediately notify any clients already subscribed (e.g. via auto-subscribe
	// on page load) so their UI flips to "running" before the first walk tick.
	e.broadcastPlanEvent(jobID, PlanEvent{})

	go func() {
		result, err := e.PlanJobStream(e.appCtx, jobID, func(files, dirs int) {
			e.broadcastPlanEvent(jobID, PlanEvent{Files: files, Folders: dirs})
		})

		e.planMu.Lock()
		delete(e.planActive, jobID)
		e.planMu.Unlock()

		var evt PlanEvent
		if err != nil {
			evt = PlanEvent{Error: err.Error()}
		} else {
			evt = PlanEvent{Done: true, Result: result}
		}
		e.broadcastPlanEvent(jobID, evt)
	}()
	return nil
}

// SubscribePlan returns a channel that receives PlanEvents for the given job,
// and an unsubscribe function that must be called when the caller is done.
//
// If a completed plan result is already stored, a done event is delivered
// immediately and the channel is closed. Otherwise the channel stays open,
// receiving progress events as they arrive (whether the plan is already running
// or starts in the future). Call the returned function to deregister early
// (e.g. on client disconnect).
func (e *Engine) SubscribePlan(jobID string) (<-chan PlanEvent, func()) {
	ch := make(chan PlanEvent, 64)

	e.planMu.Lock()

	// If a result is already stored, deliver it immediately and skip registration.
	e.storedPlansMu.Lock()
	result := e.storedPlans[jobID]
	e.storedPlansMu.Unlock()

	// Register for live events (whether a plan is active now or starts later).
	e.planSubs[jobID] = append(e.planSubs[jobID], ch)

	if result != nil {
		// Deliver the stored result immediately, but keep the channel open so
		// future plans and dismissed events flow through the same connection.
		ch <- PlanEvent{Done: true, Result: result}
	} else if e.planActive[jobID] {
		// Deliver the latest progress so the subscriber immediately shows current
		// counts rather than starting from zero.
		ch <- e.planProgress[jobID]
	}
	e.planMu.Unlock()

	unsub := func() {
		e.planMu.Lock()
		defer e.planMu.Unlock()
		subs := e.planSubs[jobID]
		for i, s := range subs {
			if s == ch {
				e.planSubs[jobID] = append(subs[:i], subs[i+1:]...)
				break
			}
		}
	}

	return ch, unsub
}

// ClearStoredPlan removes any cached plan for the job and notifies subscribers.
func (e *Engine) ClearStoredPlan(jobID string) {
	e.storedPlansMu.Lock()
	delete(e.storedPlans, jobID)
	e.storedPlansMu.Unlock()
	e.broadcastPlanEvent(jobID, PlanEvent{Dismissed: true})
}

func (e *Engine) UpdateStoredPlanAction(jobID, remotePath, action string) {
	e.storedPlansMu.Lock()
	defer e.storedPlansMu.Unlock()
	plan := e.storedPlans[jobID]
	if plan == nil {
		return
	}
	for i := range plan.Files {
		f := &plan.Files[i]
		if f.RemotePath != remotePath || f.Action == action {
			continue
		}
		if f.Action == "copy" {
			plan.ToCopy--
			plan.ToSkip++
		} else {
			plan.ToSkip--
			plan.ToCopy++
		}
		f.Action = action
		return
	}
}

func (e *Engine) broadcastPlanEvent(jobID string, evt PlanEvent) {
	e.planMu.Lock()
	// Track the latest progress event so late subscribers receive current counts.
	if !evt.Done && !evt.Dismissed && evt.Error == "" {
		e.planProgress[jobID] = evt
	} else {
		delete(e.planProgress, jobID)
	}
	subs := make([]chan PlanEvent, len(e.planSubs[jobID]))
	copy(subs, e.planSubs[jobID])
	e.planMu.Unlock()

	for _, ch := range subs {
		select {
		case ch <- evt:
		default:
		}
	}
}

// Wait blocks until all in-flight runWithPlan calls have finished.
// Call this during graceful shutdown after cancelling the context.
func (e *Engine) Wait() {
	e.runWG.Wait()
}

func (e *Engine) runWithPlan(ctx context.Context, jobID string, plan *PlanOutput) error {
	e.mu.Lock()
	if e.active[jobID] {
		e.mu.Unlock()
		return ErrJobAlreadyRunning
	}
	jobCtx, cancelJob := context.WithCancel(ctx)
	e.active[jobID] = true
	e.cancelFuncs[jobID] = cancelJob
	e.runWG.Add(1)
	e.mu.Unlock()

	defer func() {
		cancelJob()
		e.mu.Lock()
		delete(e.active, jobID)
		delete(e.activeRunIDs, jobID)
		delete(e.cancelFuncs, jobID)
		e.mu.Unlock()
		e.runWG.Done()
	}()

	job, err := e.jobs.Get(jobID)
	if err != nil || job == nil {
		return fmt.Errorf("load job %s: %w", jobID, err)
	}

	src, err := e.sources.Get(job.SourceID)
	if err != nil || src == nil {
		return fmt.Errorf("load source %s: %w", job.SourceID, err)
	}

	source, err := e.makeSource(job, src)
	if err != nil {
		return fmt.Errorf("build source: %w", err)
	}

	run := &db.Run{JobID: jobID, Status: db.RunStatusRunning}
	if err := e.runs.Create(run); err != nil {
		return fmt.Errorf("create run: %w", err)
	}
	e.mu.Lock()
	e.activeRunIDs[jobID] = run.ID
	e.mu.Unlock()
	slog.Info("job started", "job_name", job.Name, "job_id", jobID, "run_id", run.ID)

	// Create transfer records for all plan files, preserving plan order.
	batch := make([]*db.Transfer, 0, len(plan.Files))
	for _, pf := range plan.Files {
		t := &db.Transfer{
			RunID:      run.ID,
			RemotePath: pf.RemotePath,
			LocalPath:  pf.LocalPath,
			SizeBytes:  pf.SizeBytes,
			Status:     db.TransferStatusPending,
		}
		if pf.PreviousCommitHash != "" {
			t.PreviousCommitHash = &pf.PreviousCommitHash
		}
		if pf.CommitHash != "" {
			t.CurrentCommitHash = &pf.CommitHash
		}
		batch = append(batch, t)
	}
	if err := e.transfers.CreateBatch(batch); err != nil {
		return fmt.Errorf("create transfer records: %w", err)
	}
	transferByPath := make(map[string]*db.Transfer, len(batch))
	for _, t := range batch {
		transferByPath[t.RemotePath] = t
	}

	var totalSizeBytes int64
	for _, pf := range plan.Files {
		if pf.Action == "copy" {
			totalSizeBytes += pf.SizeBytes
		}
	}
	if err := e.runs.UpdateTotalSize(run.ID, totalSizeBytes); err != nil {
		slog.Error("update run total size", "run_id", run.ID, "err", err)
	}

	// Pre-process skip/error entries before calling Sync (which only sees "copy").
	initialSkipped := 0
	initialFailed := 0
	for _, pf := range plan.Files {
		t := transferByPath[pf.RemotePath]
		switch pf.Action {
		case "skip":
			if t != nil {
				if err := e.transfers.UpdateStatus(t.ID, db.TransferStatusSkipped, nil, nil); err != nil {
					slog.Error("update transfer status", "transfer_id", t.ID, "err", err)
				}
				if pf.CommitHash != "" {
					if err := e.transfers.UpdateCurrentCommitHash(t.ID, pf.CommitHash); err != nil {
						slog.Error("update transfer commit hash", "transfer_id", t.ID, "err", err)
					}
				}
			}
			initialSkipped++
		case "error":
			if t != nil {
				errMsg := pf.Error
				if err := e.transfers.UpdateStatus(t.ID, db.TransferStatusFailed, &errMsg, nil); err != nil {
					slog.Error("update transfer status", "transfer_id", t.ID, "err", err)
				}
				e.broker.Publish(run.ID, sse.Event{
					RunID: run.ID, TransferID: t.ID,
					RemotePath: pf.RemotePath,
					Status:     db.TransferStatusFailed, Error: errMsg,
				})
			}
			initialFailed++
		}
	}
	if err := e.runs.UpdateCounts(run.ID, len(plan.Files), 0, initialSkipped, initialFailed); err != nil {
		slog.Error("update run counts", "run_id", run.ID, "err", err)
	}
	// Notify clients now that transfers are created and skips pre-processed.
	// Any fetch triggered by this event will see the correct initial state.
	e.broker.Publish(jobID, sse.Event{RunID: run.ID, Status: "started"})

	var (
		mu     sync.Mutex
		copied int
		failed = initialFailed // starts seeded with plan-time error count
	)

	onEvent := func(ev TransferEvent) {
		t := transferByPath[ev.RemotePath]
		switch ev.Kind {
		case TransferEventRetrying:
		if t != nil {
			var pct float64
			if ev.SizeBytes > 0 {
				pct = float64(ev.BytesXferred) / float64(ev.SizeBytes) * 100
			}
			e.broker.Publish(run.ID, sse.Event{
				RunID: run.ID, TransferID: t.ID,
				RemotePath:   ev.RemotePath,
				SizeBytes:    ev.SizeBytes,
				BytesXferred: ev.BytesXferred,
				Percent:      pct,
				Status:       "retrying",
			})
		}
	case TransferEventStarted:
			if t != nil {
				if err := e.transfers.UpdateStatus(t.ID, db.TransferStatusInProgress, nil, nil); err != nil {
					slog.Error("update transfer status", "transfer_id", t.ID, "err", err)
				}
				e.broker.Publish(run.ID, sse.Event{
					RunID: run.ID, TransferID: t.ID,
					RemotePath: ev.RemotePath,
					Status:     db.TransferStatusInProgress,
				})
			}
		case TransferEventProgress:
			if t != nil {
				if err := e.transfers.UpdateProgress(t.ID, ev.BytesXferred); err != nil {
					slog.Error("update transfer progress", "transfer_id", t.ID, "err", err)
				}
				var pct float64
				if ev.SizeBytes > 0 {
					pct = float64(ev.BytesXferred) / float64(ev.SizeBytes) * 100
				}
				e.broker.Publish(run.ID, sse.Event{
					RunID: run.ID, TransferID: t.ID,
					RemotePath:   ev.RemotePath,
					SizeBytes:    ev.SizeBytes,
					BytesXferred: ev.BytesXferred,
					Percent:      pct,
					SpeedBPS:     ev.SpeedBPS,
					Status:       db.TransferStatusInProgress,
				})
			}
		case TransferEventDone:
			if t != nil {
				if err := e.transfers.UpdateStatus(t.ID, db.TransferStatusDone, nil, ev.DurationMs); err != nil {
					slog.Error("update transfer status", "transfer_id", t.ID, "err", err)
				}
				if ev.CommitHash != nil {
					if err := e.transfers.UpdateCurrentCommitHash(t.ID, *ev.CommitHash); err != nil {
						slog.Error("update transfer commit hash", "transfer_id", t.ID, "err", err)
					}
				}
				e.broker.Publish(run.ID, sse.Event{
					RunID: run.ID, TransferID: t.ID,
					RemotePath:   ev.RemotePath,
					SizeBytes:    ev.SizeBytes,
					BytesXferred: ev.SizeBytes,
					Percent:      100,
					Status:       db.TransferStatusDone,
				})
			}
			if ev.SyncState != nil {
				ev.SyncState.JobID = jobID
				if err := e.syncState.Upsert(ev.SyncState); err != nil {
					slog.Error("upsert sync state", "path", ev.RemotePath, "err", err)
				}
			}
			mu.Lock()
			copied++
			if err := e.runs.UpdateCounts(run.ID, len(plan.Files), copied, initialSkipped, failed); err != nil {
				slog.Error("update run counts", "run_id", run.ID, "err", err)
			}
			mu.Unlock()
		case TransferEventFailed:
			if t != nil {
				errMsg := ev.Error
				if err := e.transfers.UpdateStatus(t.ID, db.TransferStatusFailed, &errMsg, nil); err != nil {
					slog.Error("update transfer status", "transfer_id", t.ID, "err", err)
				}
				e.broker.Publish(run.ID, sse.Event{
					RunID: run.ID, TransferID: t.ID,
					RemotePath: ev.RemotePath,
					SizeBytes:  ev.SizeBytes,
					Status:     db.TransferStatusFailed,
					Error:      ev.Error,
				})
			}
			mu.Lock()
			failed++
			if err := e.runs.UpdateCounts(run.ID, len(plan.Files), copied, initialSkipped, failed); err != nil {
				slog.Error("update run counts", "run_id", run.ID, "err", err)
			}
			mu.Unlock()
		}
	}

	syncErr := source.Sync(jobCtx, SyncInput{Plan: plan, OnEvent: onEvent})

	// Mark any transfers still pending/in_progress as not_copied.
	if err := e.transfers.MarkPendingNotCopied(run.ID); err != nil {
		slog.Error("mark pending transfers not_copied", "run_id", run.ID, "err", err)
	}

	mu.Lock()
	finalCopied := copied
	finalFailed := failed
	mu.Unlock()

	var runErr error
	if syncErr != nil {
		runErr = syncErr
	} else if finalFailed > 0 {
		if finalCopied == 0 {
			runErr = failedTransferError{failed: finalFailed}
		} else {
			runErr = partialTransferError{completed: finalCopied, failed: finalFailed}
		}
	}

	finalStatus := db.RunStatusCompleted
	var finalErrMsg *string
	if e.appCtx.Err() != nil {
		finalStatus = db.RunStatusServerStopped
	} else if jobCtx.Err() != nil {
		finalStatus = db.RunStatusCanceled
	} else if runErr != nil {
		var partial partialTransferError
		if errors.As(runErr, &partial) {
			finalStatus = db.RunStatusPartial
		} else {
			finalStatus = db.RunStatusFailed
		}
		s := runErr.Error()
		finalErrMsg = &s
	} else if plan.ToCopy == 0 {
		finalStatus = db.RunStatusNothingToSync
	}
	if err := e.runs.UpdateStatus(run.ID, finalStatus, finalErrMsg); err != nil {
		slog.Error("update run status", "run_id", run.ID, "err", err)
	}

	// Prune file_state entries for files no longer present on the remote.
	// Only when the walk completed — skip on cancel or server stop.
	if finalStatus == db.RunStatusCompleted || finalStatus == db.RunStatusNothingToSync || finalStatus == db.RunStatusPartial {
		knownPaths := make([]string, len(plan.Files))
		for i, f := range plan.Files {
			knownPaths[i] = f.RemotePath
		}
		if pruned, err := e.syncState.PruneStale(jobID, knownPaths); err != nil {
			slog.Error("prune stale sync state", "job_id", jobID, "err", err)
		} else if pruned > 0 {
			slog.Info("pruned stale sync state", "job_id", jobID, "count", pruned)
		}
	}
	slog.Info("job finished", "job_name", job.Name, "job_id", jobID, "run_id", run.ID, "status", finalStatus)
	e.broker.Publish(run.ID, sse.Event{RunID: run.ID, RunStatus: finalStatus})
	e.broker.Close(run.ID)
	e.broker.Publish(jobID, sse.Event{RunID: run.ID, Status: "run_finished", RunStatus: finalStatus})
	return runErr
}

// CancelJob cancels the currently running job with the given ID.
// Returns an error if no run is active for that job.
func (e *Engine) CancelJob(jobID string) error {
	e.mu.Lock()
	cancel, ok := e.cancelFuncs[jobID]
	runID := e.activeRunIDs[jobID]
	e.mu.Unlock()
	if !ok {
		return fmt.Errorf("job %s is not running", jobID)
	}
	// Notify all clients immediately so they can show a cancelling state
	// before the run actually stops and the final status event is published.
	if runID != "" {
		e.broker.Publish(runID, sse.Event{RunID: runID, RunStatus: "canceling"})
	}
	cancel()
	return nil
}

// sortPlanFiles sorts files to match the plan tree view order: depth-first,
// folders before files at each level, both groups alpha-sorted by name.
func sortPlanFiles(files []PlanFile, remotePath string) []PlanFile {
	base := strings.TrimSuffix(remotePath, "/")
	out := make([]PlanFile, len(files))
	copy(out, files)
	sort.SliceStable(out, func(i, j int) bool {
		relI := strings.TrimPrefix(out[i].RemotePath, base+"/")
		relJ := strings.TrimPrefix(out[j].RemotePath, base+"/")
		segsI := strings.Split(relI, "/")
		segsJ := strings.Split(relJ, "/")
		for k := 0; k < len(segsI) && k < len(segsJ); k++ {
			iIsFolder := k < len(segsI)-1
			jIsFolder := k < len(segsJ)-1
			if segsI[k] == segsJ[k] {
				continue
			}
			if iIsFolder != jIsFolder {
				return iIsFolder
			}
			return segsI[k] < segsJ[k]
		}
		return len(segsI) < len(segsJ)
	})
	return out
}

// makePruner returns a shouldDescend callback for WalkWithProgress that skips
// directories that cannot contain files matching any include path filter.
// Returns nil (visit everything) when includePathFilters is empty.
func makePruner(base string, includePathFilters []string) func(string) bool {
	if len(includePathFilters) == 0 {
		return nil
	}
	prefixes := make([]string, len(includePathFilters))
	for i, subdir := range includePathFilters {
		prefixes[i] = base + "/" + strings.Trim(subdir, "/")
	}
	return func(dir string) bool {
		for _, prefix := range prefixes {
			// dir is an ancestor: need to pass through to reach the target
			if strings.HasPrefix(prefix, dir+"/") || prefix == dir {
				return true
			}
			// dir is inside the target: all contents are relevant
			if strings.HasPrefix(dir, prefix+"/") {
				return true
			}
		}
		return false
	}
}
