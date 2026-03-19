package sync

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/rnikoopour/go-ftpes/internal/crypto"
	"github.com/rnikoopour/go-ftpes/internal/db"
	"github.com/rnikoopour/go-ftpes/internal/ftpes"
	"github.com/rnikoopour/go-ftpes/internal/sse"
)

// ErrJobAlreadyRunning is returned by RunJob when a run for the job is already active.
var ErrJobAlreadyRunning = fmt.Errorf("job is already running")

// ErrPlanAlreadyRunning is returned by StartPlan when a plan for the job is already active.
var ErrPlanAlreadyRunning = fmt.Errorf("plan is already running")

// PlanEvent is a progress or terminal event broadcast to plan SSE subscribers.
type PlanEvent struct {
	Files   int         `json:"files"`
	Folders int         `json:"folders"`
	Done    bool        `json:"done"`
	Error   string      `json:"error"`
	Result  *PlanResult `json:"result,omitempty"`
}

type Engine struct {
	connections *db.ConnectionRepository
	jobs        *db.JobRepository
	runs        *db.RunRepository
	transfers   *db.TransferRepository
	fileState   *db.FileStateRepository
	encKey      []byte
	broker      *sse.Broker
	appCtx      context.Context // cancelled on server shutdown

	mu            sync.Mutex
	active        map[string]bool // jobID → running
	cancelFuncs   map[string]context.CancelFunc
	storedPlans   map[string]*PlanResult
	storedPlansMu sync.Mutex
	runWG         sync.WaitGroup // tracks all in-flight runWithPlan calls

	planMu     sync.Mutex
	planActive map[string]bool
	planSubs   map[string][]chan PlanEvent
}

func NewEngine(
	connections *db.ConnectionRepository,
	jobs *db.JobRepository,
	runs *db.RunRepository,
	transfers *db.TransferRepository,
	fileState *db.FileStateRepository,
	encKey []byte,
	broker *sse.Broker,
	appCtx context.Context,
) *Engine {
	return &Engine{
		connections: connections,
		jobs:        jobs,
		runs:        runs,
		transfers:   transfers,
		fileState:   fileState,
		encKey:      encKey,
		broker:      broker,
		appCtx:      appCtx,
		active:      make(map[string]bool),
		cancelFuncs: make(map[string]context.CancelFunc),
		storedPlans: make(map[string]*PlanResult),
		planActive:  make(map[string]bool),
		planSubs:    make(map[string][]chan PlanEvent),
	}
}

// PlanFile describes what would happen to a single remote file if the job ran.
type PlanFile struct {
	RemotePath string    `json:"remote_path"`
	LocalPath  string    `json:"local_path"`
	SizeBytes  int64     `json:"size_bytes"`
	MTime      time.Time `json:"mtime"`
	Action     string    `json:"action"` // "copy" | "skip"
}

// PlanResult is the full output of a dry-run plan.
type PlanResult struct {
	Files    []PlanFile `json:"files"`
	ToCopy   int        `json:"to_copy"`
	ToSkip   int        `json:"to_skip"`
}

// PlanJob connects to the FTPES server, walks the remote tree, and returns
// which files would be copied or skipped — without downloading anything.
func (e *Engine) PlanJob(ctx context.Context, jobID string) (*PlanResult, error) {
	return e.PlanJobStream(ctx, jobID, nil)
}

// PlanJobStream is like PlanJob but calls progress(files, dirs) after each
// file or directory is discovered during the remote walk.
func (e *Engine) PlanJobStream(ctx context.Context, jobID string, progress func(files, dirs int)) (*PlanResult, error) {
	job, err := e.jobs.Get(jobID)
	if err != nil || job == nil {
		return nil, fmt.Errorf("load job %s: %w", jobID, err)
	}

	conn, err := e.connections.Get(job.ConnectionID)
	if err != nil || conn == nil {
		return nil, fmt.Errorf("load connection %s: %w", job.ConnectionID, err)
	}

	password, err := crypto.Decrypt(e.encKey, conn.Password)
	if err != nil {
		return nil, fmt.Errorf("decrypt password: %w", err)
	}

	client, err := ftpes.Dial(conn.Host, conn.Port, conn.SkipTLSVerify, conn.EnableEPSV)
	if err != nil {
		return nil, fmt.Errorf("dial: %w", err)
	}
	defer client.Close()

	if err := client.Login(conn.Username, password); err != nil {
		return nil, fmt.Errorf("login: %w", err)
	}

	cb := progress
	if cb == nil {
		cb = func(_, _ int) {}
	}
	remoteFiles, err := client.WalkWithProgress(job.RemotePath, cb)
	if err != nil {
		return nil, fmt.Errorf("walk %s: %w", job.RemotePath, err)
	}

	result := &PlanResult{Files: make([]PlanFile, 0, len(remoteFiles))}
	for _, f := range remoteFiles {
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		if !applyFilters(f.Path, job.RemotePath, job.IncludeFilters, job.ExcludeFilters) {
			continue
		}
		state, _ := e.fileState.Get(jobID, f.Path)
		action := "copy"
		if Matches(state, f) {
			action = "skip"
			result.ToSkip++
		} else {
			result.ToCopy++
		}
		result.Files = append(result.Files, PlanFile{
			RemotePath: f.Path,
			LocalPath:  finalPath(job.LocalDest, job.RemotePath, f.Path),
			SizeBytes:  f.Size,
			MTime:      f.MTime,
			Action:     action,
		})
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
	plan, err := e.PlanJobStream(ctx, jobID, nil)
	if err != nil {
		return fmt.Errorf("plan: %w", err)
	}
	// discard stored copy — we use the result directly
	e.storedPlansMu.Lock()
	delete(e.storedPlans, jobID)
	e.storedPlansMu.Unlock()
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
		subs := e.planSubs[jobID]
		delete(e.planSubs, jobID)
		e.planMu.Unlock()

		var evt PlanEvent
		if err != nil {
			evt = PlanEvent{Error: err.Error()}
		} else {
			evt = PlanEvent{Done: true, Result: result}
		}
		for _, ch := range subs {
			select {
			case ch <- evt:
			default:
			}
			close(ch)
		}
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

	if result != nil {
		e.planMu.Unlock()
		ch <- PlanEvent{Done: true, Result: result}
		close(ch)
		return ch, func() {}
	}

	// Register for live events (whether a plan is active now or starts later).
	e.planSubs[jobID] = append(e.planSubs[jobID], ch)

	// If a plan is already running, push an immediate event so the subscriber
	// knows to show the "running" state before the next progress tick arrives.
	if e.planActive[jobID] {
		ch <- PlanEvent{}
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

func (e *Engine) broadcastPlanEvent(jobID string, evt PlanEvent) {
	e.planMu.Lock()
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

func (e *Engine) runWithPlan(ctx context.Context, jobID string, plan *PlanResult) error {
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
		delete(e.cancelFuncs, jobID)
		e.mu.Unlock()
		e.runWG.Done()
	}()

	job, err := e.jobs.Get(jobID)
	if err != nil || job == nil {
		return fmt.Errorf("load job %s: %w", jobID, err)
	}

	conn, err := e.connections.Get(job.ConnectionID)
	if err != nil || conn == nil {
		return fmt.Errorf("load connection %s: %w", job.ConnectionID, err)
	}

	run := &db.Run{JobID: jobID, Status: "running"}
	if err := e.runs.Create(run); err != nil {
		return fmt.Errorf("create run: %w", err)
	}
	// Notify all clients watching this job that a run has started.
	e.broker.Publish(jobID, sse.Event{RunID: run.ID, Status: "started"})

	runErr := e.executeRun(jobCtx, job, conn, run, plan)

	finalStatus := "completed"
	if e.appCtx.Err() != nil {
		finalStatus = "server_stopped"
	} else if jobCtx.Err() != nil {
		finalStatus = "canceled"
	} else if runErr != nil {
		finalStatus = "failed"
	}
	if err := e.runs.UpdateStatus(run.ID, finalStatus); err != nil {
		slog.Error("update run status", "run_id", run.ID, "err", err)
	}
	e.broker.Publish(run.ID, sse.Event{RunID: run.ID, RunStatus: finalStatus})
	e.broker.Close(run.ID)
	return runErr
}

// CancelJob cancels the currently running job with the given ID.
// Returns an error if no run is active for that job.
func (e *Engine) CancelJob(jobID string) error {
	e.mu.Lock()
	cancel, ok := e.cancelFuncs[jobID]
	e.mu.Unlock()
	if !ok {
		return fmt.Errorf("job %s is not running", jobID)
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

func (e *Engine) executeRun(ctx context.Context, job *db.SyncJob, conn *db.Connection, run *db.Run, plan *PlanResult) error {
	password, err := crypto.Decrypt(e.encKey, conn.Password)
	if err != nil {
		return fmt.Errorf("decrypt password: %w", err)
	}

	if err := ensureStagingDir(job.LocalDest); err != nil {
		return fmt.Errorf("staging dir: %w", err)
	}

	orderedFiles := sortPlanFiles(plan.Files, job.RemotePath)

	// Create transfer records for all plan files; track which need downloading.
	type transferEntry struct {
		transfer *db.Transfer
		remote   ftpes.RemoteFile
		skip     bool
	}
	entries := make([]transferEntry, 0, len(orderedFiles))
	for _, pf := range orderedFiles {
		remote := ftpes.RemoteFile{Path: pf.RemotePath, Size: pf.SizeBytes, MTime: pf.MTime}
		initialStatus := "pending"
		if pf.Action == "skip" {
			initialStatus = "skipped"
		}
		t := &db.Transfer{
			RunID:      run.ID,
			RemotePath: pf.RemotePath,
			LocalPath:  pf.LocalPath,
			SizeBytes:  pf.SizeBytes,
			Status:     initialStatus,
		}
		if err := e.transfers.Create(t); err != nil {
			return fmt.Errorf("create transfer record: %w", err)
		}
		entries = append(entries, transferEntry{transfer: t, remote: remote, skip: pf.Action == "skip"})
	}

	var totalSizeBytes int64
	for _, pf := range orderedFiles {
		if pf.Action == "copy" {
			totalSizeBytes += pf.SizeBytes
		}
	}
	if err := e.runs.UpdateTotalSize(run.ID, totalSizeBytes); err != nil {
		slog.Error("update run total size", "run_id", run.ID, "err", err)
	}
	if err := e.runs.UpdateCounts(run.ID, len(entries), 0, 0, 0); err != nil {
		slog.Error("update run counts", "run_id", run.ID, "err", err)
	}

	concurrency := max(job.Concurrency, 1)

	sem := make(chan struct{}, concurrency)
	var (
		mu      sync.Mutex
		copied  int
		skipped int
		failed  int
		wg      sync.WaitGroup
	)

	for _, entry := range entries {
		if ctx.Err() != nil {
			break
		}

		ent := entry
		sem <- struct{}{}

		wg.Go(func() {
			defer func() { <-sem }()

			if ent.skip {
				mu.Lock()
				skipped++
				_ = e.runs.UpdateCounts(run.ID, len(entries), copied, skipped, failed)
				mu.Unlock()
				return
			}

			// Each goroutine dials its own connection — FTP is not safe for
			// concurrent use on a single connection (PASV/RETR responses interleave).
			// tryOnce wraps dial+login+download so defer c.Close() fires on every exit path.
			tryOnce := func() error {
				c, err := ftpes.Dial(conn.Host, conn.Port, conn.SkipTLSVerify, conn.EnableEPSV)
				if err != nil {
					return err
				}
				defer c.Close()
				if err := c.Login(conn.Username, password); err != nil {
					return err
				}
				return e.downloadFile(ctx, c, job, run.ID, ent.transfer, ent.remote)
			}

			maxAttempts := max(job.RetryAttempts, 1)
			var lastErr error
			for attempt := 1; attempt <= maxAttempts; attempt++ {
				if ctx.Err() != nil {
					lastErr = ctx.Err()
					break
				}
				if attempt > 1 {
					slog.Warn("retrying transfer", "path", ent.remote.Path, "attempt", attempt, "err", lastErr)
					select {
					case <-time.After(time.Duration(job.RetryDelaySeconds) * time.Second):
					case <-ctx.Done():
						lastErr = ctx.Err()
					}
					if ctx.Err() != nil {
						break
					}
				}
				if lastErr = tryOnce(); lastErr == nil {
					break
				}
				if ctx.Err() != nil {
					break
				}
			}

			if lastErr != nil {
				slog.Error("transfer failed", "path", ent.remote.Path, "err", lastErr)
				errMsg := lastErr.Error()
				if errors.Is(lastErr, context.Canceled) {
					if e.appCtx.Err() != nil {
						errMsg = "canceled by server"
					} else {
						errMsg = "canceled by client"
					}
				}
				_ = e.transfers.UpdateStatus(ent.transfer.ID, "failed", &errMsg, nil)
				e.broker.Publish(run.ID, sse.Event{
					RunID: run.ID, TransferID: ent.transfer.ID,
					RemotePath: ent.remote.Path, SizeBytes: ent.remote.Size,
					Status: "failed", Error: errMsg,
				})
				mu.Lock()
				failed++
				_ = e.runs.UpdateCounts(run.ID, len(entries), copied, skipped, failed)
				mu.Unlock()
				return
			}

			mu.Lock()
			copied++
			_ = e.runs.UpdateCounts(run.ID, len(entries), copied, skipped, failed)
			mu.Unlock()
		})
	}

	wg.Wait()

	if failed > 0 {
		return fmt.Errorf("%d file(s) failed to transfer", failed)
	}
	return nil
}

func (e *Engine) downloadFile(
	ctx context.Context,
	client ftpes.Client,
	job *db.SyncJob,
	runID string,
	t *db.Transfer,
	remote ftpes.RemoteFile,
) error {
	stage := stagingPath(job.LocalDest, remote.Path)

	f, err := os.Create(stage)
	if err != nil {
		return fmt.Errorf("create staging file: %w", err)
	}

	start := time.Now()

	pr2, pw := newPipe()
	pr := newProgressReader(pr2, remote.Size, func(bytesRead int64) {
		_ = e.transfers.UpdateProgress(t.ID, bytesRead)

		var pct float64
		if remote.Size > 0 {
			pct = float64(bytesRead) / float64(remote.Size) * 100
		}
		elapsed := time.Since(start).Seconds()
		var speed float64
		if elapsed > 0 {
			speed = float64(bytesRead) / elapsed
		}
		e.broker.Publish(runID, sse.Event{
			RunID: runID, TransferID: t.ID,
			RemotePath:   remote.Path,
			SizeBytes:    remote.Size,
			BytesXferred: bytesRead,
			Percent:      pct,
			SpeedBPS:     speed,
			Status:       "in_progress",
		})
	})

	dlDone := make(chan error, 1)
	go func() {
		dlDone <- client.Download(remote.Path, pw)
		pw.Close()
	}()

	_, copyErr := copyWithContext(ctx, f, pr)
	if copyErr != nil {
		// Signal the download goroutine to stop: closing the pipe reader
		// causes any pending pw.Write() to return an error immediately.
		pr2.CloseWithError(copyErr)
	}
	dlErr := <-dlDone
	f.Close()

	var downloadErr error
	if copyErr != nil {
		downloadErr = copyErr
	} else if dlErr != nil {
		downloadErr = dlErr
	}

	if downloadErr != nil {
		os.Remove(stage)
		return downloadErr
	}

	durationMs := time.Since(start).Milliseconds()
	dst := finalPath(job.LocalDest, job.RemotePath, remote.Path)
	if err := atomicMove(stage, dst); err != nil {
		os.Remove(stage)
		return err
	}

	_ = e.fileState.Upsert(&db.FileState{
		JobID:      job.ID,
		RemotePath: remote.Path,
		SizeBytes:  remote.Size,
		MTime:      remote.MTime,
		CopiedAt:   time.Now().UTC(),
	})

	_ = e.transfers.UpdateStatus(t.ID, "done", nil, &durationMs)
	e.broker.Publish(runID, sse.Event{
		RunID: runID, TransferID: t.ID,
		RemotePath:   remote.Path,
		SizeBytes:    remote.Size,
		BytesXferred: remote.Size,
		Percent:      100,
		Status:       "done",
	})
	return nil
}
