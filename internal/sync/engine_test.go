package sync

import (
	"context"
	"fmt"
	"testing"

	"github.com/rnikoopour/mediocresync/internal/db"
	"github.com/rnikoopour/mediocresync/internal/sse"
)

// mockSource is a test double for Source.
type mockSource struct {
	planResult *PlanOutput
	planErr    error
	syncFunc   func(ctx context.Context, in SyncInput)
	syncErr    error
}

func (m *mockSource) Plan(ctx context.Context, in PlanInput) (*PlanOutput, error) {
	return m.planResult, m.planErr
}

func (m *mockSource) Sync(ctx context.Context, in SyncInput) error {
	if m.syncFunc != nil {
		m.syncFunc(ctx, in)
	}
	return m.syncErr
}

// openTestEngine opens an in-memory database and returns an Engine wired to it,
// along with a created job ID and source ID ready for use in tests.
func openTestEngine(t *testing.T) (*Engine, string) {
	t.Helper()
	sqlDB, err := db.Open(":memory:")
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	t.Cleanup(func() { sqlDB.Close() })

	sources := db.NewSourceRepository(sqlDB)
	gitRepos := db.NewGitRepoRepository(sqlDB)
	jobs := db.NewJobRepository(sqlDB)
	runs := db.NewRunRepository(sqlDB)
	transfers := db.NewTransferRepository(sqlDB)
	syncState := db.NewSyncStateRepository(sqlDB)
	broker := sse.NewBroker()

	src := &db.Source{Type: db.SourceTypeFTPES, Host: "test", Port: 21}
	if err := sources.Create(src); err != nil {
		t.Fatalf("create source: %v", err)
	}

	job := &db.SyncJob{
		Name:          "test-job",
		SourceID:      src.ID,
		LocalDest:     t.TempDir(),
		IntervalUnit:  "hours",
		IntervalValue: 1,
	}
	if err := jobs.Create(job); err != nil {
		t.Fatalf("create job: %v", err)
	}

	eng := NewEngine(sources, gitRepos, jobs, runs, transfers, syncState, make([]byte, 32), broker, context.Background())
	return eng, job.ID
}

// TestRunWithPlan_1Copy298Skip verifies that when 1 file is copied and 298 are
// skipped, the run record reflects exactly those counts with status "completed".
// This test would have caught the skipped-count-overwritten bug.
func TestRunWithPlan_1Copy298Skip(t *testing.T) {
	eng, jobID := openTestEngine(t)

	copyPath := "/remote/file.txt"

	plan := &PlanOutput{ToCopy: 1, ToSkip: 298}
	plan.Files = append(plan.Files, PlanFile{
		RemotePath: copyPath,
		LocalPath:  "/local/file.txt",
		Action:     "copy",
	})
	for i := 0; i < 298; i++ {
		plan.Files = append(plan.Files, PlanFile{
			RemotePath: fmt.Sprintf("/remote/skip%d.txt", i),
			LocalPath:  fmt.Sprintf("/local/skip%d.txt", i),
			Action:     "skip",
		})
	}

	mock := &mockSource{
		planResult: plan,
		syncFunc: func(ctx context.Context, in SyncInput) {
			in.OnEvent(TransferEvent{
				Kind:       TransferEventDone,
				RemotePath: copyPath,
				SizeBytes:  0,
			})
		},
	}

	eng.sourceFactory = func(*db.SyncJob, *db.Source) (Source, error) {
		return mock, nil
	}

	if err := eng.runWithPlan(context.Background(), jobID, plan); err != nil {
		t.Fatalf("runWithPlan: %v", err)
	}

	runs, err := eng.runs.ListByJob(jobID)
	if err != nil || len(runs) == 0 {
		t.Fatalf("list runs: %v (count=%d)", err, len(runs))
	}
	run := runs[0]

	if run.CopiedFiles != 1 {
		t.Errorf("CopiedFiles = %d, want 1", run.CopiedFiles)
	}
	if run.SkippedFiles != 298 {
		t.Errorf("SkippedFiles = %d, want 298", run.SkippedFiles)
	}
	if run.FailedFiles != 0 {
		t.Errorf("FailedFiles = %d, want 0", run.FailedFiles)
	}
	if run.Status != db.RunStatusCompleted {
		t.Errorf("Status = %q, want %q", run.Status, db.RunStatusCompleted)
	}
}
