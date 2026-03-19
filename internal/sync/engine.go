package sync

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"sync"
	"time"

	"github.com/rnikoopour/go-ftpes/internal/crypto"
	"github.com/rnikoopour/go-ftpes/internal/db"
	"github.com/rnikoopour/go-ftpes/internal/ftpes"
	"github.com/rnikoopour/go-ftpes/internal/sse"
)

// ErrJobAlreadyRunning is returned by RunJob when a run for the job is already active.
var ErrJobAlreadyRunning = fmt.Errorf("job is already running")

type Engine struct {
	connections *db.ConnectionRepository
	jobs        *db.JobRepository
	runs        *db.RunRepository
	transfers   *db.TransferRepository
	fileState   *db.FileStateRepository
	encKey      []byte
	broker      *sse.Broker

	mu     sync.Mutex
	active map[string]bool // jobID → running
}

func NewEngine(
	connections *db.ConnectionRepository,
	jobs *db.JobRepository,
	runs *db.RunRepository,
	transfers *db.TransferRepository,
	fileState *db.FileStateRepository,
	encKey []byte,
	broker *sse.Broker,
) *Engine {
	return &Engine{
		connections: connections,
		jobs:        jobs,
		runs:        runs,
		transfers:   transfers,
		fileState:   fileState,
		encKey:      encKey,
		broker:      broker,
		active:      make(map[string]bool),
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

	client, err := ftpes.Dial(conn.Host, conn.Port, conn.SkipTLSVerify)
	if err != nil {
		return nil, fmt.Errorf("dial: %w", err)
	}
	defer client.Close()

	if err := client.Login(conn.Username, password); err != nil {
		return nil, fmt.Errorf("login: %w", err)
	}

	remoteFiles, err := client.Walk(job.RemotePath)
	if err != nil {
		return nil, fmt.Errorf("walk %s: %w", job.RemotePath, err)
	}

	result := &PlanResult{Files: make([]PlanFile, 0, len(remoteFiles))}
	for _, f := range remoteFiles {
		if ctx.Err() != nil {
			return nil, ctx.Err()
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
	return result, nil
}

// IsRunning reports whether a run for the given job is currently active.
func (e *Engine) IsRunning(jobID string) bool {
	e.mu.Lock()
	defer e.mu.Unlock()
	return e.active[jobID]
}

// RunJob executes a full sync for the given job. Returns ErrJobAlreadyRunning
// if a run for this job is already in progress.
func (e *Engine) RunJob(ctx context.Context, jobID string) error {
	e.mu.Lock()
	if e.active[jobID] {
		e.mu.Unlock()
		return ErrJobAlreadyRunning
	}
	e.active[jobID] = true
	e.mu.Unlock()

	defer func() {
		e.mu.Lock()
		delete(e.active, jobID)
		e.mu.Unlock()
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

	runErr := e.executeRun(ctx, job, conn, run)

	finalStatus := "completed"
	if runErr != nil {
		finalStatus = "failed"
	}
	if err := e.runs.UpdateStatus(run.ID, finalStatus); err != nil {
		slog.Error("update run status", "run_id", run.ID, "err", err)
	}
	e.broker.Close(run.ID)
	return runErr
}

func (e *Engine) executeRun(ctx context.Context, job *db.SyncJob, conn *db.Connection, run *db.Run) error {
	password, err := crypto.Decrypt(e.encKey, conn.Password)
	if err != nil {
		return fmt.Errorf("decrypt password: %w", err)
	}

	client, err := ftpes.Dial(conn.Host, conn.Port, conn.SkipTLSVerify)
	if err != nil {
		return fmt.Errorf("dial: %w", err)
	}
	defer client.Close()

	if err := client.Login(conn.Username, password); err != nil {
		return fmt.Errorf("login: %w", err)
	}

	remoteFiles, err := client.Walk(job.RemotePath)
	if err != nil {
		return fmt.Errorf("walk %s: %w", job.RemotePath, err)
	}

	if err := ensureStagingDir(job.LocalDest); err != nil {
		return fmt.Errorf("staging dir: %w", err)
	}

	// Create transfer records for all files upfront.
	transfers := make([]*db.Transfer, len(remoteFiles))
	for i, f := range remoteFiles {
		t := &db.Transfer{
			RunID:      run.ID,
			RemotePath: f.Path,
			LocalPath:  finalPath(job.LocalDest, job.RemotePath, f.Path),
			SizeBytes:  f.Size,
			Status:     "pending",
		}
		if err := e.transfers.Create(t); err != nil {
			return fmt.Errorf("create transfer record: %w", err)
		}
		transfers[i] = t
	}

	if err := e.runs.UpdateCounts(run.ID, len(transfers), 0, 0, 0); err != nil {
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

	for i, remote := range remoteFiles {
		if ctx.Err() != nil {
			break
		}

		t := transfers[i]
		sem <- struct{}{}

		wg.Go(func() {
			defer func() { <-sem }()

			state, _ := e.fileState.Get(job.ID, remote.Path)
			if Matches(state, remote) {
				_ = e.transfers.UpdateStatus(t.ID, "skipped", nil, nil)
				e.broker.Publish(run.ID, sse.Event{
					RunID: run.ID, TransferID: t.ID,
					RemotePath: remote.Path, SizeBytes: remote.Size,
					Status: "skipped",
				})
				mu.Lock()
				skipped++
				_ = e.runs.UpdateCounts(run.ID, len(transfers), copied, skipped, failed)
				mu.Unlock()
				return
			}

			if err := e.downloadFile(ctx, client, job, run.ID, t, remote); err != nil {
				slog.Error("download failed", "path", remote.Path, "err", err)
				errMsg := err.Error()
				_ = e.transfers.UpdateStatus(t.ID, "failed", &errMsg, nil)
				e.broker.Publish(run.ID, sse.Event{
					RunID: run.ID, TransferID: t.ID,
					RemotePath: remote.Path, SizeBytes: remote.Size,
					Status: "failed", Error: err.Error(),
				})
				mu.Lock()
				failed++
				_ = e.runs.UpdateCounts(run.ID, len(transfers), copied, skipped, failed)
				mu.Unlock()
				return
			}

			mu.Lock()
			copied++
			_ = e.runs.UpdateCounts(run.ID, len(transfers), copied, skipped, failed)
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
