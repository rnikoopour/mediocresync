package sync

import (
	"context"
	"fmt"
	"testing"

	"github.com/rnikoopour/mediocresync/internal/db"
	"github.com/rnikoopour/mediocresync/internal/sse"
)

// listenRunSSE subscribes to job-level events, waits for "started" to learn
// the run ID, subscribes to the run channel, signals runReady, then drains
// all run-level events until the broker closes the channel.
//
// Must be called in a goroutine. runReady lets syncFunc block until the
// subscriber is in place (preventing a race between emitting and subscribing).
func listenRunSSE(eng *Engine, jobID string, runReady chan<- struct{}, out chan<- []sse.Event) {
	jobCh, jobUnsub := eng.broker.Subscribe(jobID)
	defer jobUnsub()

	var runID string
	for ev := range jobCh {
		if ev.Status == "started" {
			runID = ev.RunID
			break
		}
	}
	if runID == "" {
		out <- nil
		return
	}

	runCh, runUnsub := eng.broker.Subscribe(runID)
	defer runUnsub()
	runReady <- struct{}{} // tell syncFunc it is safe to emit events

	var collected []sse.Event
	for ev := range runCh {
		collected = append(collected, ev)
	}
	out <- collected
}

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

// TestRunWithPlan_RetryThenSuccess verifies that emitting TransferEventRetrying
// followed by TransferEventDone produces correct final counts. The retrying
// event must not corrupt copied/skipped/failed tallies.
func TestRunWithPlan_RetryThenSuccess(t *testing.T) {
	eng, jobID := openTestEngine(t)

	copyPath := "/remote/file.txt"
	plan := &PlanOutput{ToCopy: 1, ToSkip: 2}
	plan.Files = append(plan.Files, PlanFile{RemotePath: copyPath, LocalPath: "/local/file.txt", Action: "copy", SizeBytes: 1000})
	for i := range 2 {
		plan.Files = append(plan.Files, PlanFile{RemotePath: fmt.Sprintf("/remote/skip%d.txt", i), LocalPath: fmt.Sprintf("/local/skip%d.txt", i), Action: "skip"})
	}

	mock := &mockSource{
		planResult: plan,
		syncFunc: func(ctx context.Context, in SyncInput) {
			in.OnEvent(TransferEvent{Kind: TransferEventRetrying, RemotePath: copyPath, SizeBytes: 1000, BytesXferred: 400})
			in.OnEvent(TransferEvent{Kind: TransferEventDone, RemotePath: copyPath, SizeBytes: 1000})
		},
	}
	eng.sourceFactory = func(*db.SyncJob, *db.Source) (Source, error) { return mock, nil }

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
	if run.SkippedFiles != 2 {
		t.Errorf("SkippedFiles = %d, want 2", run.SkippedFiles)
	}
	if run.FailedFiles != 0 {
		t.Errorf("FailedFiles = %d, want 0", run.FailedFiles)
	}
	if run.Status != db.RunStatusCompleted {
		t.Errorf("Status = %q, want %q", run.Status, db.RunStatusCompleted)
	}
}

// TestRunWithPlan_RetryThenFail verifies that a file that retries and ultimately
// fails is counted as failed, not copied.
func TestRunWithPlan_RetryThenFail(t *testing.T) {
	eng, jobID := openTestEngine(t)

	copyPath := "/remote/file.txt"
	plan := &PlanOutput{ToCopy: 1}
	plan.Files = append(plan.Files, PlanFile{RemotePath: copyPath, LocalPath: "/local/file.txt", Action: "copy", SizeBytes: 1000})

	mock := &mockSource{
		planResult: plan,
		syncFunc: func(ctx context.Context, in SyncInput) {
			in.OnEvent(TransferEvent{Kind: TransferEventRetrying, RemotePath: copyPath, SizeBytes: 1000, BytesXferred: 0})
			in.OnEvent(TransferEvent{Kind: TransferEventFailed, RemotePath: copyPath, SizeBytes: 1000, Error: "connection reset"})
		},
	}
	eng.sourceFactory = func(*db.SyncJob, *db.Source) (Source, error) { return mock, nil }

	eng.runWithPlan(context.Background(), jobID, plan) //nolint:errcheck — expected to return a failure error

	runs, err := eng.runs.ListByJob(jobID)
	if err != nil || len(runs) == 0 {
		t.Fatalf("list runs: %v (count=%d)", err, len(runs))
	}
	run := runs[0]

	if run.CopiedFiles != 0 {
		t.Errorf("CopiedFiles = %d, want 0", run.CopiedFiles)
	}
	if run.FailedFiles != 1 {
		t.Errorf("FailedFiles = %d, want 1", run.FailedFiles)
	}
	if run.Status != db.RunStatusFailed {
		t.Errorf("Status = %q, want %q", run.Status, db.RunStatusFailed)
	}
}

// TestRunWithPlan_RetrySSE verifies that TransferEventRetrying causes the broker
// to publish an SSE event with status "retrying" and the staged byte count,
// preserving the progress floor instead of resetting to 0.
func TestRunWithPlan_RetrySSE(t *testing.T) {
	eng, jobID := openTestEngine(t)

	copyPath := "/remote/file.txt"
	const fileSize = int64(1000)
	const stagedBytes = int64(400)

	plan := &PlanOutput{ToCopy: 1}
	plan.Files = append(plan.Files, PlanFile{RemotePath: copyPath, LocalPath: "/local/file.txt", Action: "copy", SizeBytes: fileSize})

	// Capture the run ID from the job-level "started" SSE event so we can
	// query the DB mid-run without waiting for the run to complete.
	runIDCh := make(chan string, 1)
	go func() {
		jobCh, unsub := eng.broker.Subscribe(jobID)
		defer unsub()
		for ev := range jobCh {
			if ev.Status == "started" {
				runIDCh <- ev.RunID
				return
			}
		}
	}()

	// retryReady is closed by syncFunc after the retry event (and its DB write)
	// are complete. retryDone is closed by the test to let syncFunc proceed.
	retryReady := make(chan struct{})
	retryDone  := make(chan struct{})
	runReady   := make(chan struct{}, 1)

	mock := &mockSource{
		planResult: plan,
		syncFunc: func(ctx context.Context, in SyncInput) {
			<-runReady
			in.OnEvent(TransferEvent{Kind: TransferEventRetrying, RemotePath: copyPath, SizeBytes: fileSize, BytesXferred: stagedBytes})
			close(retryReady)
			<-retryDone
			in.OnEvent(TransferEvent{Kind: TransferEventDone, RemotePath: copyPath, SizeBytes: fileSize})
		},
	}
	eng.sourceFactory = func(*db.SyncJob, *db.Source) (Source, error) { return mock, nil }

	out := make(chan []sse.Event, 1)
	go listenRunSSE(eng, jobID, runReady, out)

	runErr := make(chan error, 1)
	go func() { runErr <- eng.runWithPlan(context.Background(), jobID, plan) }()

	runID := <-runIDCh

	// Pause after retry and verify bytes_xferred was persisted to the DB so a
	// fresh page load can show the correct progress floor without a live SSE event.
	<-retryReady
	dbTransfers, err := eng.transfers.ListByRun(runID)
	if err != nil {
		t.Fatalf("ListByRun after retry: %v", err)
	}
	if len(dbTransfers) != 1 {
		t.Fatalf("expected 1 transfer, got %d", len(dbTransfers))
	}
	if dbTransfers[0].BytesXferred != stagedBytes {
		t.Errorf("DB bytes_xferred after retry = %d, want %d", dbTransfers[0].BytesXferred, stagedBytes)
	}
	close(retryDone)

	if err := <-runErr; err != nil {
		t.Fatalf("runWithPlan: %v", err)
	}

	events := <-out

	var retryingEv *sse.Event
	for i, ev := range events {
		if ev.Status == "retrying" {
			retryingEv = &events[i]
			break
		}
	}
	if retryingEv == nil {
		t.Fatal("no 'retrying' SSE event published")
	}
	if retryingEv.BytesXferred != stagedBytes {
		t.Errorf("retrying BytesXferred = %d, want %d", retryingEv.BytesXferred, stagedBytes)
	}
	wantPct := float64(stagedBytes) / float64(fileSize) * 100
	if retryingEv.Percent != wantPct {
		t.Errorf("retrying Percent = %f, want %f", retryingEv.Percent, wantPct)
	}
}
