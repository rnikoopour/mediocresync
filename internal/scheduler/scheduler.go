package scheduler

import (
	"context"
	"errors"
	"log/slog"
	"time"

	"github.com/rnikoopour/mediocresync/internal/db"
	"github.com/rnikoopour/mediocresync/internal/sse"
	internalsync "github.com/rnikoopour/mediocresync/internal/sync"
)

const tickInterval = time.Minute

type Scheduler struct {
	jobs   *db.JobRepository
	runs   *db.RunRepository
	engine *internalsync.Engine
	broker *sse.Broker
	stop   chan struct{}
}

func NewScheduler(jobs *db.JobRepository, runs *db.RunRepository, engine *internalsync.Engine, broker *sse.Broker) *Scheduler {
	return &Scheduler{
		jobs:   jobs,
		runs:   runs,
		engine: engine,
		broker: broker,
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
		if err := s.engine.RunJob(jobID); err != nil && !errors.Is(err, internalsync.ErrJobAlreadyRunning) {
			slog.Error("manual run failed", "job_id", jobID, "err", err)
		}
	}()
	return nil
}

func (s *Scheduler) tick(ctx context.Context) {
	enabledJobs, err := s.jobs.ListEnabled()
	if err != nil {
		slog.Error("scheduler: list enabled jobs", "err", err)
		return
	}
	for _, job := range enabledJobs {
		if isDue(job, s.lastRun(job.ID)) {
			jobID := job.ID
			go func() {
				if err := s.engine.PlanThenRun(ctx, jobID); err != nil {
					if !errors.Is(err, internalsync.ErrJobAlreadyRunning) {
						slog.Error("scheduled run failed", "job_id", jobID, "err", err)
					}
				}
			}()
		}
	}

	allJobs, err := s.jobs.List()
	if err != nil {
		slog.Error("scheduler: list all jobs for pruning", "err", err)
		return
	}
	for _, job := range allJobs {
		if job.RunRetentionDays > 0 {
			if err := s.runs.PruneForJob(job.ID, job.RunRetentionDays); err != nil {
				slog.Error("scheduler: prune run history", "job_id", job.ID, "err", err)
			} else {
				s.broker.Publish(job.ID, sse.Event{Status: "runs_pruned"})
			}
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

// isDue returns true if the job has not yet run during the current
// clock-aligned slot. Slots are computed by truncating the current time to
// the job's interval, so a 60-minute job fires at 00:00, 01:00, 02:00, etc.
// and a 30-minute job fires at 00:00, 00:30, 01:00, etc.
func isDue(job *db.SyncJob, lastRun time.Time) bool {
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

	slotStart := time.Now().UTC().Truncate(interval)
	return lastRun.Before(slotStart)
}
