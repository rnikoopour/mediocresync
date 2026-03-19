package scheduler

import (
	"context"
	"errors"
	"log/slog"
	"time"

	"github.com/rnikoopour/go-ftpes/internal/db"
	internalsync "github.com/rnikoopour/go-ftpes/internal/sync"
)

const tickInterval = time.Minute

type Scheduler struct {
	jobs   *db.JobRepository
	runs   *db.RunRepository
	engine *internalsync.Engine
	stop   chan struct{}
}

func NewScheduler(jobs *db.JobRepository, runs *db.RunRepository, engine *internalsync.Engine) *Scheduler {
	return &Scheduler{
		jobs:   jobs,
		runs:   runs,
		engine: engine,
		stop:   make(chan struct{}),
	}
}

// Start begins the scheduler in a background goroutine. It fires any overdue
// jobs immediately, then checks every minute.
func (s *Scheduler) Start(ctx context.Context) {
	go func() {
		s.tick(ctx)

		t := time.NewTicker(tickInterval)
		defer t.Stop()

		for {
			select {
			case <-t.C:
				s.tick(ctx)
			case <-s.stop:
				return
			case <-ctx.Done():
				return
			}
		}
	}()
}

// Stop shuts down the scheduler. In-flight runs are not interrupted.
func (s *Scheduler) Stop() {
	close(s.stop)
}

// TriggerNow starts a run for the given job immediately. Returns
// ErrJobAlreadyRunning if a run is already active.
func (s *Scheduler) TriggerNow(ctx context.Context, jobID string) error {
	go func() {
		if err := s.engine.RunJob(ctx, jobID); err != nil && !errors.Is(err, internalsync.ErrJobAlreadyRunning) {
			slog.Error("manual run failed", "job_id", jobID, "err", err)
		}
	}()
	return nil
}

func (s *Scheduler) tick(ctx context.Context) {
	jobs, err := s.jobs.ListEnabled()
	if err != nil {
		slog.Error("scheduler: list enabled jobs", "err", err)
		return
	}

	for _, job := range jobs {
		if isDue(job, s.lastRun(job.ID)) {
			jobID := job.ID
			go func() {
				if err := s.engine.RunJob(ctx, jobID); err != nil {
					if !errors.Is(err, internalsync.ErrJobAlreadyRunning) {
						slog.Error("scheduled run failed", "job_id", jobID, "err", err)
					}
				}
			}()
		}
	}
}

// lastRun returns the start time of the most recent run for the job, or the
// zero time if no runs exist.
func (s *Scheduler) lastRun(jobID string) time.Time {
	runs, err := s.runs.ListByJob(jobID)
	if err != nil || len(runs) == 0 {
		return time.Time{}
	}
	return runs[0].StartedAt
}

// isDue returns true if enough time has elapsed since the last run to warrant
// a new one, based on the job's interval setting.
func isDue(job *db.SyncJob, lastRun time.Time) bool {
	if lastRun.IsZero() {
		return true
	}

	var interval time.Duration
	switch job.IntervalUnit {
	case "minutes":
		interval = time.Duration(job.IntervalValue) * time.Minute
	case "hours":
		interval = time.Duration(job.IntervalValue) * time.Hour
	case "days":
		interval = time.Duration(job.IntervalValue) * 24 * time.Hour
	default:
		return false
	}

	return time.Since(lastRun) >= interval
}
