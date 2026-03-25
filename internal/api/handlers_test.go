package api

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
	"testing/fstest"
	"time"

	"github.com/rnikoopour/mediocresync/internal/crypto"
	"github.com/rnikoopour/mediocresync/internal/db"
	internalsync "github.com/rnikoopour/mediocresync/internal/sync"
	"github.com/rnikoopour/mediocresync/internal/sse"
)

var testEncKey = bytes.Repeat([]byte{0x01}, 32)

func setupRouterFull(t *testing.T) (*sql.DB, http.Handler, *db.SourceRepository, *db.JobRepository, *db.RunRepository, string) {
	t.Helper()
	database, err := db.Open(":memory:")
	if err != nil {
		t.Fatalf("open DB: %v", err)
	}
	t.Cleanup(func() { database.Close() })

	auth := db.NewAuthRepository(database)
	sources := db.NewSourceRepository(database)
	jobs := db.NewJobRepository(database)
	runs := db.NewRunRepository(database)
	transfers := db.NewTransferRepository(database)
	syncState := db.NewSyncStateRepository(database)
	broker := sse.NewBroker()
	gitRepos := db.NewGitRepoRepository(database)
	engine := internalsync.NewEngine(sources, gitRepos, jobs, runs, transfers, syncState, testEncKey, broker, context.Background())

	staticFS := fstest.MapFS{"index.html": {Data: []byte("<html></html>")}}
	router := NewRouter(context.Background(), "dev", auth, sources, gitRepos, jobs, runs, transfers, syncState, engine, broker, testEncKey, true, new(slog.LevelVar), nil, staticFS)

	w := do(t, router, "POST", "/api/auth/setup", map[string]any{
		"username": "testuser", "password": "testpass", "password_confirm": "testpass",
	})
	if w.Code != http.StatusOK {
		t.Fatalf("test setup: got %d (body: %s)", w.Code, w.Body.String())
	}
	w = do(t, router, "POST", "/api/auth/login", map[string]any{
		"username": "testuser", "password": "testpass",
	})
	if w.Code != http.StatusOK {
		t.Fatalf("test login: got %d (body: %s)", w.Code, w.Body.String())
	}
	var sessionToken string
	for _, c := range w.Result().Cookies() {
		if c.Name == sessionCookie {
			sessionToken = c.Value
			break
		}
	}
	if sessionToken == "" {
		t.Fatal("no session cookie after login")
	}

	return database, router, sources, jobs, runs, sessionToken
}

func setupRouter(t *testing.T) (http.Handler, *db.SourceRepository, *db.JobRepository, *db.RunRepository, string) {
	t.Helper()
	database, err := db.Open(":memory:")
	if err != nil {
		t.Fatalf("open DB: %v", err)
	}
	t.Cleanup(func() { database.Close() })

	auth := db.NewAuthRepository(database)
	sources := db.NewSourceRepository(database)
	jobs := db.NewJobRepository(database)
	runs := db.NewRunRepository(database)
	transfers := db.NewTransferRepository(database)
	syncState := db.NewSyncStateRepository(database)
	broker := sse.NewBroker()
	gitRepos := db.NewGitRepoRepository(database)
	engine := internalsync.NewEngine(sources, gitRepos, jobs, runs, transfers, syncState, testEncKey, broker, context.Background())

	staticFS := fstest.MapFS{"index.html": {Data: []byte("<html></html>")}}
	router := NewRouter(context.Background(), "dev", auth, sources, gitRepos, jobs, runs, transfers, syncState, engine, broker, testEncKey, true, new(slog.LevelVar), nil, staticFS)

	// Configure credentials and log in so tests can hit protected endpoints.
	w := do(t, router, "POST", "/api/auth/setup", map[string]any{
		"username": "testuser", "password": "testpass", "password_confirm": "testpass",
	})
	if w.Code != http.StatusOK {
		t.Fatalf("test setup: got %d (body: %s)", w.Code, w.Body.String())
	}
	w = do(t, router, "POST", "/api/auth/login", map[string]any{
		"username": "testuser", "password": "testpass",
	})
	if w.Code != http.StatusOK {
		t.Fatalf("test login: got %d (body: %s)", w.Code, w.Body.String())
	}
	var sessionToken string
	for _, c := range w.Result().Cookies() {
		if c.Name == sessionCookie {
			sessionToken = c.Value
			break
		}
	}
	if sessionToken == "" {
		t.Fatal("no session cookie after login")
	}

	return router, sources, jobs, runs, sessionToken
}

// do performs an HTTP request against the router. Pass a session token as the
// last argument to attach it as the session cookie for authenticated requests.
func do(t *testing.T, router http.Handler, method, path string, body any, session ...string) *httptest.ResponseRecorder {
	t.Helper()
	var buf bytes.Buffer
	if body != nil {
		_ = json.NewEncoder(&buf).Encode(body)
	}
	req := httptest.NewRequest(method, path, &buf)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if len(session) > 0 && session[0] != "" {
		req.AddCookie(&http.Cookie{Name: sessionCookie, Value: session[0]})
	}
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	return w
}

func decodeJSON(t *testing.T, w *httptest.ResponseRecorder, v any) {
	t.Helper()
	if err := json.NewDecoder(w.Body).Decode(v); err != nil {
		t.Fatalf("decode response JSON: %v (body: %s)", err, w.Body.String())
	}
}

// --- Sources ---

func TestListSourcesEmpty(t *testing.T) {
	router, _, _, _, session := setupRouter(t)
	w := do(t, router, "GET", "/api/sources", nil, session)
	if w.Code != http.StatusOK {
		t.Fatalf("got %d, want 200", w.Code)
	}
	var list []sourceResponse
	decodeJSON(t, w, &list)
	if len(list) != 0 {
		t.Errorf("expected empty list, got %d items", len(list))
	}
}

func TestCreateAndGetSource(t *testing.T) {
	router, _, _, _, session := setupRouter(t)

	w := do(t, router, "POST", "/api/sources", map[string]any{
		"name": "test", "type": "ftpes", "host": "ftp.example.com", "port": 21,
		"username": "user", "password": "secret", "skip_tls_verify": false,
	}, session)
	if w.Code != http.StatusCreated {
		t.Fatalf("create: got %d, want 201 (body: %s)", w.Code, w.Body.String())
	}

	var created sourceResponse
	decodeJSON(t, w, &created)
	if created.ID == "" {
		t.Fatal("no ID in created source")
	}
	// Password must never be in the response
	if w.Body.String() != "" {
		var raw map[string]any
		_ = json.Unmarshal([]byte(w.Body.String()), &raw)
		if _, hasPassword := raw["password"]; hasPassword {
			t.Error("password should not be in response")
		}
	}

	// GET by ID
	w2 := do(t, router, "GET", "/api/sources/"+created.ID, nil, session)
	if w2.Code != http.StatusOK {
		t.Fatalf("get: got %d, want 200", w2.Code)
	}
}

func TestCreateSourceMissingFields(t *testing.T) {
	router, _, _, _, session := setupRouter(t)
	w := do(t, router, "POST", "/api/sources", map[string]any{"name": "incomplete", "type": "ftpes"}, session)
	if w.Code != http.StatusBadRequest {
		t.Errorf("got %d, want 400", w.Code)
	}
}

func TestDeleteSource(t *testing.T) {
	router, _, _, _, session := setupRouter(t)

	w := do(t, router, "POST", "/api/sources", map[string]any{
		"name": "del", "type": "ftpes", "host": "h", "port": 21, "username": "u", "password": "p",
	}, session)
	var created sourceResponse
	decodeJSON(t, w, &created)

	w2 := do(t, router, "DELETE", "/api/sources/"+created.ID, nil, session)
	if w2.Code != http.StatusNoContent {
		t.Errorf("delete: got %d, want 204", w2.Code)
	}

	w3 := do(t, router, "GET", "/api/sources/"+created.ID, nil, session)
	if w3.Code != http.StatusNotFound {
		t.Errorf("after delete, get should return 404, got %d", w3.Code)
	}
}

func TestGetSourceNotFound(t *testing.T) {
	router, _, _, _, session := setupRouter(t)
	w := do(t, router, "GET", "/api/sources/nonexistent", nil, session)
	if w.Code != http.StatusNotFound {
		t.Errorf("got %d, want 404", w.Code)
	}
}

func TestUpdateSourcePasswordOptional(t *testing.T) {
	router, srcRepo, _, _, session := setupRouter(t)

	// Create with known password
	encrypted, _ := crypto.Encrypt(testEncKey, "original")
	src := &db.Source{Name: "c", Type: db.SourceTypeFTPES, Host: "h", Port: 21, Username: "u", Password: encrypted}
	_ = srcRepo.Create(src)

	// Update without providing a new password
	w := do(t, router, "PUT", "/api/sources/"+src.ID, map[string]any{
		"name": "updated", "type": "ftpes", "host": "h", "port": 21, "username": "u", "password": "",
	}, session)
	if w.Code != http.StatusOK {
		t.Fatalf("update: got %d (body: %s)", w.Code, w.Body.String())
	}

	// Password in DB should be unchanged
	got, _ := srcRepo.Get(src.ID)
	decrypted, err := crypto.Decrypt(testEncKey, got.Password)
	if err != nil || decrypted != "original" {
		t.Errorf("password should be unchanged, got %q (err: %v)", decrypted, err)
	}
}

// --- Jobs ---

func TestCreateAndListJobs(t *testing.T) {
	router, srcRepo, _, _, session := setupRouter(t)

	encrypted, _ := crypto.Encrypt(testEncKey, "pass")
	src := &db.Source{Name: "c", Type: db.SourceTypeFTPES, Host: "h", Port: 21, Username: "u", Password: encrypted}
	_ = srcRepo.Create(src)

	w := do(t, router, "POST", "/api/jobs", map[string]any{
		"name": "myjob", "source_id": src.ID, "remote_path": "/",
		"local_dest": "/tmp", "interval_value": 30, "interval_unit": "minutes",
		"concurrency": 3, "enabled": true,
	}, session)
	if w.Code != http.StatusCreated {
		t.Fatalf("create job: got %d (body: %s)", w.Code, w.Body.String())
	}

	var created jobResponse
	decodeJSON(t, w, &created)
	if created.Concurrency != 3 {
		t.Errorf("concurrency: got %d, want 3", created.Concurrency)
	}

	w2 := do(t, router, "GET", "/api/jobs", nil, session)
	var list []jobResponse
	decodeJSON(t, w2, &list)
	if len(list) != 1 {
		t.Errorf("list: got %d jobs, want 1", len(list))
	}
}

func TestCreateGitJobWithRepos(t *testing.T) {
	router, srcRepo, _, _, session := setupRouter(t)

	src := &db.Source{Name: "gs", Type: db.SourceTypeGit, AuthType: db.AuthTypeNone}
	_ = srcRepo.Create(src)

	w := do(t, router, "POST", "/api/jobs", map[string]any{
		"name": "gitjob", "source_id": src.ID, "local_dest": "/tmp",
		"interval_value": 1, "interval_unit": "hours", "concurrency": 2, "enabled": true,
		"git_repos": []map[string]any{
			{"url": "https://github.com/org/repo-a", "branch": "main"},
			{"url": "https://github.com/org/repo-b", "branch": "dev"},
		},
	}, session)
	if w.Code != http.StatusCreated {
		t.Fatalf("create git job: got %d (body: %s)", w.Code, w.Body.String())
	}

	var resp jobResponse
	decodeJSON(t, w, &resp)
	if len(resp.GitRepos) != 2 {
		t.Fatalf("git_repos: got %d, want 2", len(resp.GitRepos))
	}
	if resp.GitRepos[0].URL != "https://github.com/org/repo-a" || resp.GitRepos[0].Branch != "main" {
		t.Errorf("unexpected repo[0]: %+v", resp.GitRepos[0])
	}
	if resp.GitRepos[1].URL != "https://github.com/org/repo-b" || resp.GitRepos[1].Branch != "dev" {
		t.Errorf("unexpected repo[1]: %+v", resp.GitRepos[1])
	}
	for _, r := range resp.GitRepos {
		if r.ID == "" {
			t.Error("repo ID not populated in response")
		}
	}

	// GET should also return repos.
	wGet := do(t, router, "GET", "/api/jobs/"+resp.ID, nil, session)
	var getResp jobResponse
	decodeJSON(t, wGet, &getResp)
	if len(getResp.GitRepos) != 2 {
		t.Errorf("GET git_repos: got %d, want 2", len(getResp.GitRepos))
	}
}

func TestUpdateGitJobReplacesRepos(t *testing.T) {
	router, srcRepo, _, _, session := setupRouter(t)

	src := &db.Source{Name: "gs", Type: db.SourceTypeGit, AuthType: db.AuthTypeNone}
	_ = srcRepo.Create(src)

	// Create with two repos.
	wCreate := do(t, router, "POST", "/api/jobs", map[string]any{
		"name": "gitjob", "source_id": src.ID, "local_dest": "/tmp",
		"interval_value": 1, "interval_unit": "hours", "concurrency": 1, "enabled": true,
		"git_repos": []map[string]any{
			{"url": "https://github.com/org/repo-a"},
			{"url": "https://github.com/org/repo-b"},
		},
	}, session)
	var created jobResponse
	decodeJSON(t, wCreate, &created)

	// Update with a single different repo.
	wUpdate := do(t, router, "PUT", "/api/jobs/"+created.ID, map[string]any{
		"name": "gitjob", "source_id": src.ID, "local_dest": "/tmp",
		"interval_value": 1, "interval_unit": "hours", "concurrency": 1, "enabled": true,
		"git_repos": []map[string]any{
			{"url": "https://github.com/org/repo-c", "branch": "release"},
		},
	}, session)
	if wUpdate.Code != http.StatusOK {
		t.Fatalf("update job: got %d (body: %s)", wUpdate.Code, wUpdate.Body.String())
	}

	var updated jobResponse
	decodeJSON(t, wUpdate, &updated)
	if len(updated.GitRepos) != 1 {
		t.Fatalf("git_repos after update: got %d, want 1", len(updated.GitRepos))
	}
	if updated.GitRepos[0].URL != "https://github.com/org/repo-c" || updated.GitRepos[0].Branch != "release" {
		t.Errorf("unexpected repo after update: %+v", updated.GitRepos[0])
	}
}

func TestTriggerRunAlreadyRunning(t *testing.T) {
	router, srcRepo, jobRepo, _, session := setupRouter(t)

	encrypted, _ := crypto.Encrypt(testEncKey, "pass")
	src := &db.Source{Name: "c", Type: db.SourceTypeFTPES, Host: "h", Port: 21, Username: "u", Password: encrypted}
	_ = srcRepo.Create(src)
	job := &db.SyncJob{
		Name: "j", SourceID: src.ID, RemotePath: "/", LocalDest: "/tmp",
		IntervalValue: 1, IntervalUnit: "hours", Concurrency: 1, Enabled: true,
	}
	_ = jobRepo.Create(job)

	// First trigger — should return 202
	w := do(t, router, "POST", "/api/jobs/"+job.ID+"/run", nil, session)
	if w.Code != http.StatusAccepted {
		t.Errorf("first trigger: got %d, want 202", w.Code)
	}
}

// --- Runs ---

func TestListRunsEmpty(t *testing.T) {
	router, srcRepo, jobRepo, _, session := setupRouter(t)

	encrypted, _ := crypto.Encrypt(testEncKey, "pass")
	src := &db.Source{Name: "c", Type: db.SourceTypeFTPES, Host: "h", Port: 21, Username: "u", Password: encrypted}
	_ = srcRepo.Create(src)
	job := &db.SyncJob{
		Name: "j", SourceID: src.ID, RemotePath: "/", LocalDest: "/tmp",
		IntervalValue: 1, IntervalUnit: "hours", Concurrency: 1, Enabled: true,
	}
	_ = jobRepo.Create(job)

	w := do(t, router, "GET", "/api/jobs/"+job.ID+"/runs", nil, session)
	if w.Code != http.StatusOK {
		t.Fatalf("list runs: got %d", w.Code)
	}
	var list []runResponse
	decodeJSON(t, w, &list)
	if len(list) != 0 {
		t.Errorf("expected 0 runs, got %d", len(list))
	}
}

func TestGetRunNotFound(t *testing.T) {
	router, _, _, _, session := setupRouter(t)
	w := do(t, router, "GET", "/api/runs/nonexistent", nil, session)
	if w.Code != http.StatusNotFound {
		t.Errorf("got %d, want 404", w.Code)
	}
}

func TestPlanThenRunReturns202(t *testing.T) {
	router, srcRepo, jobRepo, _, session := setupRouter(t)

	encrypted, _ := crypto.Encrypt(testEncKey, "pass")
	src := &db.Source{Name: "c", Type: db.SourceTypeFTPES, Host: "h", Port: 21, Username: "u", Password: encrypted}
	_ = srcRepo.Create(src)
	job := &db.SyncJob{
		Name: "j", SourceID: src.ID, RemotePath: "/", LocalDest: "/tmp",
		IntervalValue: 1, IntervalUnit: "hours", Concurrency: 1, Enabled: true,
	}
	_ = jobRepo.Create(job)

	w := do(t, router, "POST", "/api/jobs/"+job.ID+"/planthenrun", nil, session)
	if w.Code != http.StatusAccepted {
		t.Errorf("planthenrun: got %d, want 202", w.Code)
	}
}

func TestPlanThenRunJobNotFound(t *testing.T) {
	router, _, _, _, session := setupRouter(t)

	w := do(t, router, "POST", "/api/jobs/nonexistent/planthenrun", nil, session)
	if w.Code != http.StatusNotFound {
		t.Errorf("planthenrun nonexistent job: got %d, want 404", w.Code)
	}
}

func TestRunErrorMsgPersistedOnFailure(t *testing.T) {
	_, _, srcRepo, jobRepo, runRepo, _ := setupRouterFull(t)

	encrypted, _ := crypto.Encrypt(testEncKey, "pass")
	src := &db.Source{Name: "c", Type: db.SourceTypeFTPES, Host: "h", Port: 21, Username: "u", Password: encrypted}
	_ = srcRepo.Create(src)
	job := &db.SyncJob{
		Name: "j", SourceID: src.ID, RemotePath: "/", LocalDest: "/tmp",
		IntervalValue: 1, IntervalUnit: "hours", Concurrency: 1, Enabled: true,
	}
	_ = jobRepo.Create(job)

	run := &db.Run{JobID: job.ID, Status: db.RunStatusRunning}
	if err := runRepo.Create(run); err != nil {
		t.Fatalf("create run: %v", err)
	}

	errMsg := "dial tcp: connection refused"
	if err := runRepo.UpdateStatus(run.ID, db.RunStatusFailed, &errMsg); err != nil {
		t.Fatalf("UpdateStatus: %v", err)
	}

	got, err := runRepo.Get(run.ID)
	if err != nil {
		t.Fatalf("Get run: %v", err)
	}
	if got.ErrorMsg == nil {
		t.Fatal("expected error_msg to be set, got nil")
	}
	if *got.ErrorMsg != errMsg {
		t.Errorf("error_msg: got %q, want %q", *got.ErrorMsg, errMsg)
	}
}

func TestUpdateJobPrunesOldRuns(t *testing.T) {
	database, router, srcRepo, jobRepo, runRepo, session := setupRouterFull(t)

	encrypted, _ := crypto.Encrypt(testEncKey, "pass")
	src := &db.Source{Name: "c", Type: db.SourceTypeFTPES, Host: "h", Port: 21, Username: "u", Password: encrypted}
	_ = srcRepo.Create(src)
	job := &db.SyncJob{
		Name: "j", SourceID: src.ID, RemotePath: "/", LocalDest: "/tmp",
		IntervalValue: 1, IntervalUnit: "hours", Concurrency: 1, Enabled: true,
	}
	_ = jobRepo.Create(job)

	// Insert an old run and a recent run directly
	oldRun := &db.Run{JobID: job.ID, Status: db.RunStatusCompleted}
	_ = runRepo.Create(oldRun)
	_, _ = database.Exec(`UPDATE runs SET started_at=? WHERE id=?`,
		time.Now().UTC().AddDate(0, 0, -3).Format(time.RFC3339), oldRun.ID)

	recentRun := &db.Run{JobID: job.ID, Status: db.RunStatusCompleted}
	_ = runRepo.Create(recentRun)

	// Update job with 1-day retention — should prune the old run
	w := do(t, router, "PUT", "/api/jobs/"+job.ID, map[string]any{
		"name": job.Name, "source_id": job.SourceID, "remote_path": job.RemotePath,
		"local_dest": job.LocalDest, "interval_value": job.IntervalValue, "interval_unit": job.IntervalUnit,
		"concurrency": job.Concurrency, "enabled": job.Enabled, "run_retention_days": 1,
	}, session)
	if w.Code != http.StatusOK {
		t.Fatalf("update job: got %d (body: %s)", w.Code, w.Body.String())
	}

	if got, _ := runRepo.Get(oldRun.ID); got != nil {
		t.Error("old run should have been pruned on save")
	}
	if got, _ := runRepo.Get(recentRun.ID); got == nil {
		t.Error("recent run should not have been pruned")
	}
}
