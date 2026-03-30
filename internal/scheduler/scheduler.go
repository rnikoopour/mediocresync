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

// lastRun returns the most recent time that should be considered "covered" for
// scheduling purposes:
//   - If the most recent run has finished, use FinishedAt (so a long run that
//     finished at 04:30 covers the 04:00 slot and won't re-fire until 05:00).
//   - If the run is still in progress, use time.Now() so isDue returns false
//     until the run completes and the next slot is reached.
func (s *Scheduler) lastRun(jobID string) time.Time {
	runs, err := s.runs.ListByJob(jobID)
	if err != nil || len(runs) == 0 {
		return time.Time{}
	}
	r := runs[0]
	if r.FinishedAt != nil {
		return *r.FinishedAt
	}
	// Run is still in progress — treat now as the coverage time so we don't
	// attempt to start a second run for the current slot.
	return time.Now()
}

// isDue returns true if the job has not yet run during the current
// clock-aligned slot. Slots are anchored to local midnight so e.g. a
// 12-hour job fires at 00:00 and 12:00 local time.
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

	slotStart := currentSlotStart(time.Now(), interval)
	return lastRun.Before(slotStart)
}

// currentSlotStart returns the start of the current slot anchored to local
// midnight. Sub-day intervals (minutes, hours) are divided from today's local
// midnight; day-based intervals are anchored to a fixed reference date
// (2000-01-01 local) so multi-day cadences stay consistent.
func currentSlotStart(now time.Time, interval time.Duration) time.Time {
	if interval <= 0 {
		return now
	}
	if interval >= 24*time.Hour {
		ref := time.Date(2000, 1, 1, 0, 0, 0, 0, now.Location())
		elapsed := now.Sub(ref)
		return ref.Add((elapsed / interval) * interval)
	}
	y, m, d := now.Date()
	midnight := time.Date(y, m, d, 0, 0, 0, 0, now.Location())
	elapsed := now.Sub(midnight)
	return midnight.Add((elapsed / interval) * interval)
}
