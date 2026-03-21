package db

import (
	"testing"
	"time"
)

func openTestDB(t *testing.T) *ConnectionRepository {
	t.Helper()
	database, err := Open(":memory:")
	if err != nil {
		t.Fatalf("open test DB: %v", err)
	}
	t.Cleanup(func() { database.Close() })
	return NewConnectionRepository(database)
}

func openAllRepos(t *testing.T) (*ConnectionRepository, *JobRepository, *RunRepository, *TransferRepository, *FileStateRepository) {
	t.Helper()
	database, err := Open(":memory:")
	if err != nil {
		t.Fatalf("open test DB: %v", err)
	}
	t.Cleanup(func() { database.Close() })
	return NewConnectionRepository(database),
		NewJobRepository(database),
		NewRunRepository(database),
		NewTransferRepository(database),
		NewFileStateRepository(database)
}

// --- ConnectionRepository ---

func TestConnectionCRUD(t *testing.T) {
	repo := openTestDB(t)

	conn := &Connection{
		Name: "test", Host: "ftp.example.com", Port: 21,
		Username: "user", Password: []byte("encrypted"), SkipTLSVerify: false,
	}

	if err := repo.Create(conn); err != nil {
		t.Fatalf("Create: %v", err)
	}
	if conn.ID == "" {
		t.Fatal("ID not set after Create")
	}

	got, err := repo.Get(conn.ID)
	if err != nil || got == nil {
		t.Fatalf("Get: %v", err)
	}
	if got.Name != conn.Name || got.Host != conn.Host {
		t.Errorf("Get returned wrong data: %+v", got)
	}

	list, err := repo.List()
	if err != nil || len(list) != 1 {
		t.Fatalf("List: got %d items, want 1 (err: %v)", len(list), err)
	}

	conn.Name = "renamed"
	if err := repo.Update(conn); err != nil {
		t.Fatalf("Update: %v", err)
	}
	got, _ = repo.Get(conn.ID)
	if got.Name != "renamed" {
		t.Errorf("Update: name not changed, got %q", got.Name)
	}

	if err := repo.Delete(conn.ID); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	got, _ = repo.Get(conn.ID)
	if got != nil {
		t.Error("Delete: record still exists")
	}
}

func TestConnectionGetNotFound(t *testing.T) {
	repo := openTestDB(t)
	got, err := repo.Get("nonexistent")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != nil {
		t.Error("expected nil for missing connection")
	}
}

// --- JobRepository ---

func TestJobCRUD(t *testing.T) {
	connRepo, jobRepo, _, _, _ := openAllRepos(t)

	conn := &Connection{Name: "c", Host: "h", Port: 21, Username: "u", Password: []byte("p")}
	_ = connRepo.Create(conn)

	job := &SyncJob{
		Name: "myjob", ConnectionID: conn.ID, RemotePath: "/",
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
	connRepo, jobRepo, runRepo, _, _ := openAllRepos(t)

	conn := &Connection{Name: "c", Host: "h", Port: 21, Username: "u", Password: []byte("p")}
	_ = connRepo.Create(conn)
	job := &SyncJob{Name: "j", ConnectionID: conn.ID, RemotePath: "/", LocalDest: "/tmp", IntervalValue: 1, IntervalUnit: "hours", Concurrency: 1, Enabled: true}
	_ = jobRepo.Create(job)

	run := &Run{JobID: job.ID, Status: "running"}
	if err := runRepo.Create(run); err != nil {
		t.Fatalf("Create run: %v", err)
	}

	if err := runRepo.UpdateCounts(run.ID, 10, 8, 1, 1); err != nil {
		t.Fatalf("UpdateCounts: %v", err)
	}
	if err := runRepo.UpdateStatus(run.ID, "completed"); err != nil {
		t.Fatalf("UpdateStatus: %v", err)
	}

	got, err := runRepo.Get(run.ID)
	if err != nil || got == nil {
		t.Fatalf("Get run: %v", err)
	}
	if got.Status != "completed" || got.TotalFiles != 10 {
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

// --- FileStateRepository ---

func TestFileStateUpsert(t *testing.T) {
	connRepo, jobRepo, _, _, fsRepo := openAllRepos(t)

	conn := &Connection{Name: "c", Host: "h", Port: 21, Username: "u", Password: []byte("p")}
	_ = connRepo.Create(conn)
	job := &SyncJob{Name: "j", ConnectionID: conn.ID, RemotePath: "/", LocalDest: "/tmp", IntervalValue: 1, IntervalUnit: "hours", Concurrency: 1, Enabled: true}
	_ = jobRepo.Create(job)

	state := &FileState{
		JobID: job.ID, RemotePath: "/reports/jan.csv",
		SizeBytes: 1024, MTime: time.Now().UTC().Truncate(time.Second),
		CopiedAt: time.Now().UTC(),
	}

	if err := fsRepo.Upsert(state); err != nil {
		t.Fatalf("Upsert: %v", err)
	}

	got, err := fsRepo.Get(job.ID, "/reports/jan.csv")
	if err != nil || got == nil {
		t.Fatalf("Get file state: %v", err)
	}
	if got.SizeBytes != 1024 {
		t.Errorf("got SizeBytes %d, want 1024", got.SizeBytes)
	}

	// Update via upsert
	state.SizeBytes = 2048
	_ = fsRepo.Upsert(state)
	got, _ = fsRepo.Get(job.ID, "/reports/jan.csv")
	if got.SizeBytes != 2048 {
		t.Errorf("upsert should update existing record, got %d", got.SizeBytes)
	}

	if err := fsRepo.DeleteByJob(job.ID); err != nil {
		t.Fatalf("DeleteByJob: %v", err)
	}
	got, _ = fsRepo.Get(job.ID, "/reports/jan.csv")
	if got != nil {
		t.Error("file state should be deleted")
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

			connRepo := NewConnectionRepository(database)
			jobRepo := NewJobRepository(database)
			runRepo := NewRunRepository(database)
			transferRepo := NewTransferRepository(database)

			conn := &Connection{Name: "c", Host: "h", Port: 21, Username: "u", Password: []byte("p")}
			_ = connRepo.Create(conn)
			job := &SyncJob{Name: "j", ConnectionID: conn.ID, RemotePath: "/", LocalDest: "/tmp", IntervalValue: 1, IntervalUnit: "hours", Concurrency: 1, Enabled: true}
			_ = jobRepo.Create(job)

			run := &Run{JobID: job.ID, Status: "completed"}
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

func TestFileStateGetMissing(t *testing.T) {
	connRepo, jobRepo, _, _, fsRepo := openAllRepos(t)
	conn := &Connection{Name: "c", Host: "h", Port: 21, Username: "u", Password: []byte("p")}
	_ = connRepo.Create(conn)
	job := &SyncJob{Name: "j", ConnectionID: conn.ID, RemotePath: "/", LocalDest: "/tmp", IntervalValue: 1, IntervalUnit: "hours", Concurrency: 1, Enabled: true}
	_ = jobRepo.Create(job)

	got, err := fsRepo.Get(job.ID, "/nonexistent.csv")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != nil {
		t.Error("expected nil for missing file state")
	}
}
