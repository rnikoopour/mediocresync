package db

import (
	"testing"
	"time"
)

func openTestDB(t *testing.T) *SourceRepository {
	t.Helper()
	database, err := Open(":memory:")
	if err != nil {
		t.Fatalf("open test DB: %v", err)
	}
	t.Cleanup(func() { database.Close() })
	return NewSourceRepository(database)
}

func openAllRepos(t *testing.T) (*SourceRepository, *JobRepository, *RunRepository, *TransferRepository, *SyncStateRepository) {
	t.Helper()
	database, err := Open(":memory:")
	if err != nil {
		t.Fatalf("open test DB: %v", err)
	}
	t.Cleanup(func() { database.Close() })
	return NewSourceRepository(database),
		NewJobRepository(database),
		NewRunRepository(database),
		NewTransferRepository(database),
		NewSyncStateRepository(database)
}

// --- SourceRepository ---

func TestSourceCRUD(t *testing.T) {
	repo := openTestDB(t)

	src := &Source{
		Name: "test", Type: SourceTypeFTPES, Host: "ftp.example.com", Port: 21,
		Username: "user", Password: []byte("encrypted"), SkipTLSVerify: false,
	}

	if err := repo.Create(src); err != nil {
		t.Fatalf("Create: %v", err)
	}
	if src.ID == "" {
		t.Fatal("ID not set after Create")
	}

	got, err := repo.Get(src.ID)
	if err != nil || got == nil {
		t.Fatalf("Get: %v", err)
	}
	if got.Name != src.Name || got.Host != src.Host {
		t.Errorf("Get returned wrong data: %+v", got)
	}

	list, err := repo.List()
	if err != nil || len(list) != 1 {
		t.Fatalf("List: got %d items, want 1 (err: %v)", len(list), err)
	}

	src.Name = "renamed"
	if err := repo.Update(src); err != nil {
		t.Fatalf("Update: %v", err)
	}
	got, _ = repo.Get(src.ID)
	if got.Name != "renamed" {
		t.Errorf("Update: name not changed, got %q", got.Name)
	}

	if err := repo.Delete(src.ID); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	got, _ = repo.Get(src.ID)
	if got != nil {
		t.Error("Delete: record still exists")
	}
}

func TestSourceGetNotFound(t *testing.T) {
	repo := openTestDB(t)
	got, err := repo.Get("nonexistent")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != nil {
		t.Error("expected nil for missing source")
	}
}

// --- GitRepoRepository ---

func openGitRepoRepos(t *testing.T) (*SourceRepository, *JobRepository, *GitRepoRepository) {
	t.Helper()
	database, err := Open(":memory:")
	if err != nil {
		t.Fatalf("open test DB: %v", err)
	}
	t.Cleanup(func() { database.Close() })
	return NewSourceRepository(database), NewJobRepository(database), NewGitRepoRepository(database)
}

func TestGitRepoListByJobEmpty(t *testing.T) {
	_, _, gitRepos := openGitRepoRepos(t)
	repos, err := gitRepos.ListByJob("nonexistent-job")
	if err != nil {
		t.Fatalf("ListByJob: %v", err)
	}
	if len(repos) != 0 {
		t.Errorf("expected 0 repos, got %d", len(repos))
	}
}

func TestGitRepoReplaceForJob(t *testing.T) {
	srcRepo, jobRepo, gitRepos := openGitRepoRepos(t)

	src := &Source{Name: "s", Type: SourceTypeGit, AuthType: AuthTypeNone}
	_ = srcRepo.Create(src)
	job := &SyncJob{Name: "j", SourceID: src.ID, LocalDest: "/tmp", IntervalValue: 1, IntervalUnit: "hours", Concurrency: 1, Enabled: true}
	_ = jobRepo.Create(job)

	// Initial insert.
	initial := []*GitRepo{
		{URL: "https://github.com/org/repo-a", Branch: "main"},
		{URL: "https://github.com/org/repo-b"},
	}
	if err := gitRepos.ReplaceForJob(job.ID, initial); err != nil {
		t.Fatalf("ReplaceForJob (insert): %v", err)
	}

	list, err := gitRepos.ListByJob(job.ID)
	if err != nil {
		t.Fatalf("ListByJob: %v", err)
	}
	if len(list) != 2 {
		t.Fatalf("expected 2 repos, got %d", len(list))
	}
	// Results are ordered by URL.
	if list[0].URL != "https://github.com/org/repo-a" || list[0].Branch != "main" {
		t.Errorf("unexpected repo[0]: %+v", list[0])
	}
	// Branch defaults to "main" when empty.
	if list[1].URL != "https://github.com/org/repo-b" || list[1].Branch != "main" {
		t.Errorf("unexpected repo[1]: %+v", list[1])
	}
	for _, r := range list {
		if r.ID == "" {
			t.Error("ID not set after insert")
		}
		if r.JobID != job.ID {
			t.Errorf("JobID: got %q, want %q", r.JobID, job.ID)
		}
	}

	// Replace with a different set.
	replacement := []*GitRepo{
		{URL: "https://github.com/org/repo-c", Branch: "dev"},
	}
	if err := gitRepos.ReplaceForJob(job.ID, replacement); err != nil {
		t.Fatalf("ReplaceForJob (replace): %v", err)
	}

	list, err = gitRepos.ListByJob(job.ID)
	if err != nil {
		t.Fatalf("ListByJob after replace: %v", err)
	}
	if len(list) != 1 {
		t.Fatalf("expected 1 repo after replace, got %d", len(list))
	}
	if list[0].URL != "https://github.com/org/repo-c" || list[0].Branch != "dev" {
		t.Errorf("unexpected repo after replace: %+v", list[0])
	}

	// Replace with empty list clears all.
	if err := gitRepos.ReplaceForJob(job.ID, nil); err != nil {
		t.Fatalf("ReplaceForJob (clear): %v", err)
	}
	list, _ = gitRepos.ListByJob(job.ID)
	if len(list) != 0 {
		t.Errorf("expected 0 repos after clear, got %d", len(list))
	}
}

// --- JobRepository ---

func TestJobCRUD(t *testing.T) {
	srcRepo, jobRepo, _, _, _ := openAllRepos(t)

	src := &Source{Name: "c", Type: SourceTypeFTPES, Host: "h", Port: 21, Username: "u", Password: []byte("p")}
	_ = srcRepo.Create(src)

	job := &SyncJob{
		Name: "myjob", SourceID: src.ID, RemotePath: "/",
		LocalDest: "/tmp/dest", IntervalValue: 30, IntervalUnit: "minutes",
		Concurrency: 2, Enabled: true,
	}
	if err := jobRepo.Create(job); err != nil {
		t.Fatalf("Create: %v", err)
	}

	got, err := jobRepo.Get(job.ID)
	if err != nil || got == nil {
		t.Fatalf("Get: %v", err)
	}
	if got.Concurrency != 2 || got.IntervalUnit != "minutes" {
		t.Errorf("unexpected job data: %+v", got)
	}

	enabled, err := jobRepo.ListEnabled()
	if err != nil || len(enabled) != 1 {
		t.Fatalf("ListEnabled: got %d, want 1", len(enabled))
	}

	job.Enabled = false
	_ = jobRepo.Update(job)
	enabled, _ = jobRepo.ListEnabled()
	if len(enabled) != 0 {
		t.Error("disabled job should not appear in ListEnabled")
	}
}

// --- RunRepository ---

func TestRunLifecycle(t *testing.T) {
	srcRepo, jobRepo, runRepo, _, _ := openAllRepos(t)

	src := &Source{Name: "c", Type: SourceTypeFTPES, Host: "h", Port: 21, Username: "u", Password: []byte("p")}
	_ = srcRepo.Create(src)
	job := &SyncJob{Name: "j", SourceID: src.ID, RemotePath: "/", LocalDest: "/tmp", IntervalValue: 1, IntervalUnit: "hours", Concurrency: 1, Enabled: true}
	_ = jobRepo.Create(job)

	run := &Run{JobID: job.ID, Status: RunStatusRunning}
	if err := runRepo.Create(run); err != nil {
		t.Fatalf("Create run: %v", err)
	}

	if err := runRepo.UpdateCounts(run.ID, 10, 8, 1, 1, 1024); err != nil {
		t.Fatalf("UpdateCounts: %v", err)
	}
	if err := runRepo.UpdateStatus(run.ID, RunStatusCompleted, nil); err != nil {
		t.Fatalf("UpdateStatus: %v", err)
	}

	got, err := runRepo.Get(run.ID)
	if err != nil || got == nil {
		t.Fatalf("Get run: %v", err)
	}
	if got.Status != RunStatusCompleted || got.TotalFiles != 10 {
		t.Errorf("unexpected run data: %+v", got)
	}
	if got.FinishedAt == nil {
		t.Error("FinishedAt should be set after completed status")
	}

	list, err := runRepo.ListByJob(job.ID)
	if err != nil || len(list) != 1 {
		t.Fatalf("ListByJob: got %d, want 1", len(list))
	}
}

// TestTransferStatusConstraint verifies that all defined transfer status values
// satisfy the CHECK constraint. This test would have caught the missing
// not_copied and canceled statuses before they hit production.
func TestTransferStatusConstraint(t *testing.T) {
	srcRepo, jobRepo, runRepo, transferRepo, _ := openAllRepos(t)

	src := &Source{Name: "c", Type: SourceTypeFTPES, Host: "h", Port: 21, Username: "u", Password: []byte("p")}
	_ = srcRepo.Create(src)
	job := &SyncJob{Name: "j", SourceID: src.ID, RemotePath: "/", LocalDest: "/tmp", IntervalValue: 1, IntervalUnit: "hours", Concurrency: 1, Enabled: true}
	_ = jobRepo.Create(job)
	run := &Run{JobID: job.ID, Status: RunStatusRunning}
	_ = runRepo.Create(run)

	statuses := []string{
		TransferStatusPending,
		TransferStatusInProgress,
		TransferStatusDone,
		TransferStatusSkipped,
		TransferStatusFailed,
		TransferStatusNotCopied,
		TransferStatusCanceled,
	}

	for _, status := range statuses {
		t.Run(status, func(t *testing.T) {
			transfers := []*Transfer{{RunID: run.ID, RemotePath: "/f", LocalPath: "/l", Status: status}}
			if err := transferRepo.CreateBatch(transfers); err != nil {
				t.Fatalf("CreateBatch with status %q: %v", status, err)
			}
			// Also exercise UpdateStatus to confirm the constraint allows the value.
			if err := transferRepo.UpdateStatus(transfers[0].ID, status, nil, nil); err != nil {
				t.Fatalf("UpdateStatus to %q: %v", status, err)
			}
		})
	}

	// MarkPendingNotCopied must not fail — exercises the not_copied status path.
	if err := transferRepo.MarkPendingNotCopied(run.ID); err != nil {
		t.Fatalf("MarkPendingNotCopied: %v", err)
	}
}

// --- SyncStateRepository ---

func TestSyncStateUpsert(t *testing.T) {
	srcRepo, jobRepo, _, _, ssRepo := openAllRepos(t)

	src := &Source{Name: "c", Type: SourceTypeFTPES, Host: "h", Port: 21, Username: "u", Password: []byte("p")}
	_ = srcRepo.Create(src)
	job := &SyncJob{Name: "j", SourceID: src.ID, RemotePath: "/", LocalDest: "/tmp", IntervalValue: 1, IntervalUnit: "hours", Concurrency: 1, Enabled: true}
	_ = jobRepo.Create(job)

	mtime := time.Now().UTC().Truncate(time.Second)
	state := &SyncState{
		JobID: job.ID, RemotePath: "/reports/jan.csv",
		SizeBytes: 1024, MTime: &mtime,
		CopiedAt: time.Now().UTC(),
	}

	if err := ssRepo.Upsert(state); err != nil {
		t.Fatalf("Upsert: %v", err)
	}

	got, err := ssRepo.Get(job.ID, "/reports/jan.csv")
	if err != nil || got == nil {
		t.Fatalf("Get sync state: %v", err)
	}
	if got.SizeBytes != 1024 {
		t.Errorf("got SizeBytes %d, want 1024", got.SizeBytes)
	}

	// Update via upsert
	state.SizeBytes = 2048
	_ = ssRepo.Upsert(state)
	got, _ = ssRepo.Get(job.ID, "/reports/jan.csv")
	if got.SizeBytes != 2048 {
		t.Errorf("upsert should update existing record, got %d", got.SizeBytes)
	}

	if err := ssRepo.DeleteByJob(job.ID); err != nil {
		t.Fatalf("DeleteByJob: %v", err)
	}
	got, _ = ssRepo.Get(job.ID, "/reports/jan.csv")
	if got != nil {
		t.Error("sync state should be deleted")
	}
}

// --- RunRepository.PruneForJob ---

func TestPruneForJob(t *testing.T) {
	cases := []struct {
		name          string
		retentionDays int
		runAgeDays    int
		wantPruned    bool
	}{
		{"zero retention is noop", 0, 3, false},
		{"old run pruned", 1, 3, true},
		{"run within window kept", 7, 3, false},
		{"recent run kept", 1, 0, false},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			database, err := Open(":memory:")
			if err != nil {
				t.Fatalf("open test DB: %v", err)
			}
			t.Cleanup(func() { database.Close() })

			srcRepo := NewSourceRepository(database)
			jobRepo := NewJobRepository(database)
			runRepo := NewRunRepository(database)
			transferRepo := NewTransferRepository(database)

			src := &Source{Name: "c", Type: SourceTypeFTPES, Host: "h", Port: 21, Username: "u", Password: []byte("p")}
			_ = srcRepo.Create(src)
			job := &SyncJob{Name: "j", SourceID: src.ID, RemotePath: "/", LocalDest: "/tmp", IntervalValue: 1, IntervalUnit: "hours", Concurrency: 1, Enabled: true}
			_ = jobRepo.Create(job)

			run := &Run{JobID: job.ID, Status: RunStatusCompleted}
			if err := runRepo.Create(run); err != nil {
				t.Fatalf("create run: %v", err)
			}
			_, err = database.Exec(`UPDATE runs SET started_at=? WHERE id=?`,
				formatTime(time.Now().UTC().AddDate(0, 0, -tc.runAgeDays)), run.ID)
			if err != nil {
				t.Fatalf("backdate run: %v", err)
			}

			transfer := &Transfer{RunID: run.ID, RemotePath: "/f.txt", LocalPath: "/tmp/f.txt", SizeBytes: 100, Status: "done"}
			if err := transferRepo.Create(transfer); err != nil {
				t.Fatalf("create transfer: %v", err)
			}

			if err := runRepo.PruneForJob(job.ID, tc.retentionDays); err != nil {
				t.Fatalf("PruneForJob: %v", err)
			}

			got, _ := runRepo.Get(run.ID)
			if tc.wantPruned && got != nil {
				t.Error("run should have been pruned")
			}
			if !tc.wantPruned && got == nil {
				t.Error("run should not have been pruned")
			}

			if tc.wantPruned {
				if transfers, _ := transferRepo.ListByRun(run.ID); len(transfers) != 0 {
					t.Errorf("transfers should be cascade-deleted, got %d", len(transfers))
				}
			}
		})
	}
}

func TestSyncStatePruneStale(t *testing.T) {
	srcRepo, jobRepo, _, _, ssRepo := openAllRepos(t)
	src := &Source{Name: "c", Type: SourceTypeFTPES, Host: "h", Port: 21, Username: "u", Password: []byte("p")}
	_ = srcRepo.Create(src)
	job := &SyncJob{Name: "j", SourceID: src.ID, RemotePath: "/", LocalDest: "/tmp", IntervalValue: 1, IntervalUnit: "hours", Concurrency: 1, Enabled: true}
	_ = jobRepo.Create(job)

	mtime := time.Now()
	for _, path := range []string{"/a.csv", "/b.csv", "/c.csv"} {
		_ = ssRepo.Upsert(&SyncState{JobID: job.ID, RemotePath: path, SizeBytes: 1, MTime: &mtime, CopiedAt: time.Now()})
	}

	// Only /a.csv and /c.csv are still on the remote — /b.csv was deleted.
	if _, err := ssRepo.PruneStale(job.ID, []string{"/a.csv", "/c.csv"}); err != nil {
		t.Fatalf("PruneStale: %v", err)
	}

	if got, _ := ssRepo.Get(job.ID, "/a.csv"); got == nil {
		t.Error("/a.csv should be retained")
	}
	if got, _ := ssRepo.Get(job.ID, "/c.csv"); got == nil {
		t.Error("/c.csv should be retained")
	}
	if got, _ := ssRepo.Get(job.ID, "/b.csv"); got != nil {
		t.Error("/b.csv should have been pruned")
	}
}

func TestSyncStateGetMissing(t *testing.T) {
	srcRepo, jobRepo, _, _, ssRepo := openAllRepos(t)
	src := &Source{Name: "c", Type: SourceTypeFTPES, Host: "h", Port: 21, Username: "u", Password: []byte("p")}
	_ = srcRepo.Create(src)
	job := &SyncJob{Name: "j", SourceID: src.ID, RemotePath: "/", LocalDest: "/tmp", IntervalValue: 1, IntervalUnit: "hours", Concurrency: 1, Enabled: true}
	_ = jobRepo.Create(job)

	got, err := ssRepo.Get(job.ID, "/nonexistent.csv")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != nil {
		t.Error("expected nil for missing sync state")
	}
}
