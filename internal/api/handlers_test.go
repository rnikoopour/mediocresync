package api

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"testing/fstest"

	"github.com/rnikoopour/go-ftpes/internal/crypto"
	"github.com/rnikoopour/go-ftpes/internal/db"
	internalsync "github.com/rnikoopour/go-ftpes/internal/sync"
	"github.com/rnikoopour/go-ftpes/internal/sse"
)

var testEncKey = bytes.Repeat([]byte{0x01}, 32)

func setupRouter(t *testing.T) (http.Handler, *db.ConnectionRepository, *db.JobRepository, *db.RunRepository) {
	t.Helper()
	database, err := db.Open(":memory:")
	if err != nil {
		t.Fatalf("open DB: %v", err)
	}
	t.Cleanup(func() { database.Close() })

	connections := db.NewConnectionRepository(database)
	jobs := db.NewJobRepository(database)
	runs := db.NewRunRepository(database)
	transfers := db.NewTransferRepository(database)
	fileState := db.NewFileStateRepository(database)
	broker := sse.NewBroker()
	engine := internalsync.NewEngine(connections, jobs, runs, transfers, fileState, testEncKey, broker, context.Background())

	staticFS := fstest.MapFS{"index.html": {Data: []byte("<html></html>")}}
	router := NewRouter(context.Background(), connections, jobs, runs, transfers, fileState, engine, broker, testEncKey, true, staticFS)
	return router, connections, jobs, runs
}

func do(t *testing.T, router http.Handler, method, path string, body any) *httptest.ResponseRecorder {
	t.Helper()
	var buf bytes.Buffer
	if body != nil {
		_ = json.NewEncoder(&buf).Encode(body)
	}
	req := httptest.NewRequest(method, path, &buf)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
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

// --- Connections ---

func TestListConnectionsEmpty(t *testing.T) {
	router, _, _, _ := setupRouter(t)
	w := do(t, router, "GET", "/api/connections", nil)
	if w.Code != http.StatusOK {
		t.Fatalf("got %d, want 200", w.Code)
	}
	var list []connectionResponse
	decodeJSON(t, w, &list)
	if len(list) != 0 {
		t.Errorf("expected empty list, got %d items", len(list))
	}
}

func TestCreateAndGetConnection(t *testing.T) {
	router, _, _, _ := setupRouter(t)

	w := do(t, router, "POST", "/api/connections", map[string]any{
		"name": "test", "host": "ftp.example.com", "port": 21,
		"username": "user", "password": "secret", "skip_tls_verify": false,
	})
	if w.Code != http.StatusCreated {
		t.Fatalf("create: got %d, want 201 (body: %s)", w.Code, w.Body.String())
	}

	var created connectionResponse
	decodeJSON(t, w, &created)
	if created.ID == "" {
		t.Fatal("no ID in created connection")
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
	w2 := do(t, router, "GET", "/api/connections/"+created.ID, nil)
	if w2.Code != http.StatusOK {
		t.Fatalf("get: got %d, want 200", w2.Code)
	}
}

func TestCreateConnectionMissingFields(t *testing.T) {
	router, _, _, _ := setupRouter(t)
	w := do(t, router, "POST", "/api/connections", map[string]any{"name": "incomplete"})
	if w.Code != http.StatusBadRequest {
		t.Errorf("got %d, want 400", w.Code)
	}
}

func TestDeleteConnection(t *testing.T) {
	router, _, _, _ := setupRouter(t)

	w := do(t, router, "POST", "/api/connections", map[string]any{
		"name": "del", "host": "h", "port": 21, "username": "u", "password": "p",
	})
	var created connectionResponse
	decodeJSON(t, w, &created)

	w2 := do(t, router, "DELETE", "/api/connections/"+created.ID, nil)
	if w2.Code != http.StatusNoContent {
		t.Errorf("delete: got %d, want 204", w2.Code)
	}

	w3 := do(t, router, "GET", "/api/connections/"+created.ID, nil)
	if w3.Code != http.StatusNotFound {
		t.Errorf("after delete, get should return 404, got %d", w3.Code)
	}
}

func TestGetConnectionNotFound(t *testing.T) {
	router, _, _, _ := setupRouter(t)
	w := do(t, router, "GET", "/api/connections/nonexistent", nil)
	if w.Code != http.StatusNotFound {
		t.Errorf("got %d, want 404", w.Code)
	}
}

func TestUpdateConnectionPasswordOptional(t *testing.T) {
	router, connRepo, _, _ := setupRouter(t)

	// Create with known password
	encrypted, _ := crypto.Encrypt(testEncKey, "original")
	conn := &db.Connection{Name: "c", Host: "h", Port: 21, Username: "u", Password: encrypted}
	_ = connRepo.Create(conn)

	// Update without providing a new password
	w := do(t, router, "PUT", "/api/connections/"+conn.ID, map[string]any{
		"name": "updated", "host": "h", "port": 21, "username": "u", "password": "",
	})
	if w.Code != http.StatusOK {
		t.Fatalf("update: got %d (body: %s)", w.Code, w.Body.String())
	}

	// Password in DB should be unchanged
	got, _ := connRepo.Get(conn.ID)
	decrypted, err := crypto.Decrypt(testEncKey, got.Password)
	if err != nil || decrypted != "original" {
		t.Errorf("password should be unchanged, got %q (err: %v)", decrypted, err)
	}
}

// --- Jobs ---

func TestCreateAndListJobs(t *testing.T) {
	router, connRepo, _, _ := setupRouter(t)

	encrypted, _ := crypto.Encrypt(testEncKey, "pass")
	conn := &db.Connection{Name: "c", Host: "h", Port: 21, Username: "u", Password: encrypted}
	_ = connRepo.Create(conn)

	w := do(t, router, "POST", "/api/jobs", map[string]any{
		"name": "myjob", "connection_id": conn.ID, "remote_path": "/",
		"local_dest": "/tmp", "interval_value": 30, "interval_unit": "minutes",
		"concurrency": 3, "enabled": true,
	})
	if w.Code != http.StatusCreated {
		t.Fatalf("create job: got %d (body: %s)", w.Code, w.Body.String())
	}

	var created jobResponse
	decodeJSON(t, w, &created)
	if created.Concurrency != 3 {
		t.Errorf("concurrency: got %d, want 3", created.Concurrency)
	}

	w2 := do(t, router, "GET", "/api/jobs", nil)
	var list []jobResponse
	decodeJSON(t, w2, &list)
	if len(list) != 1 {
		t.Errorf("list: got %d jobs, want 1", len(list))
	}
}

func TestTriggerRunAlreadyRunning(t *testing.T) {
	router, connRepo, jobRepo, _ := setupRouter(t)

	encrypted, _ := crypto.Encrypt(testEncKey, "pass")
	conn := &db.Connection{Name: "c", Host: "h", Port: 21, Username: "u", Password: encrypted}
	_ = connRepo.Create(conn)
	job := &db.SyncJob{
		Name: "j", ConnectionID: conn.ID, RemotePath: "/", LocalDest: "/tmp",
		IntervalValue: 1, IntervalUnit: "hours", Concurrency: 1, Enabled: true,
	}
	_ = jobRepo.Create(job)

	// First trigger — should return 202
	w := do(t, router, "POST", "/api/jobs/"+job.ID+"/run", nil)
	if w.Code != http.StatusAccepted {
		t.Errorf("first trigger: got %d, want 202", w.Code)
	}
}

// --- Runs ---

func TestListRunsEmpty(t *testing.T) {
	router, connRepo, jobRepo, _ := setupRouter(t)

	encrypted, _ := crypto.Encrypt(testEncKey, "pass")
	conn := &db.Connection{Name: "c", Host: "h", Port: 21, Username: "u", Password: encrypted}
	_ = connRepo.Create(conn)
	job := &db.SyncJob{
		Name: "j", ConnectionID: conn.ID, RemotePath: "/", LocalDest: "/tmp",
		IntervalValue: 1, IntervalUnit: "hours", Concurrency: 1, Enabled: true,
	}
	_ = jobRepo.Create(job)

	w := do(t, router, "GET", "/api/jobs/"+job.ID+"/runs", nil)
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
	router, _, _, _ := setupRouter(t)
	w := do(t, router, "GET", "/api/runs/nonexistent", nil)
	if w.Code != http.StatusNotFound {
		t.Errorf("got %d, want 404", w.Code)
	}
}
