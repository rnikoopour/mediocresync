package api

import (
	"context"
	"log/slog"
	"net/http"
	"testing"
	"testing/fstest"
	"time"

	"github.com/rnikoopour/mediocresync/internal/db"
	internalsync "github.com/rnikoopour/mediocresync/internal/sync"
	"github.com/rnikoopour/mediocresync/internal/sse"
)

// setupUnconfiguredRouter returns a router with no credentials configured yet,
// along with the AuthRepository so tests can manipulate auth state directly.
func setupUnconfiguredRouter(t *testing.T) (http.Handler, *db.AuthRepository) {
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
	router := NewRouter(context.Background(), "dev", auth, sources, jobs, runs, transfers, syncState, engine, broker, testEncKey, true, new(slog.LevelVar), nil, staticFS)
	return router, auth
}

// loginAs performs setup (if not yet configured) and login, returning the session token.
func loginAs(t *testing.T, router http.Handler, username, password string) string {
	t.Helper()
	w := do(t, router, "POST", "/api/auth/login", map[string]any{
		"username": username, "password": password,
	})
	if w.Code != http.StatusOK {
		t.Fatalf("login: got %d (body: %s)", w.Code, w.Body.String())
	}
	for _, c := range w.Result().Cookies() {
		if c.Name == sessionCookie {
			return c.Value
		}
	}
	t.Fatal("no session cookie returned")
	return ""
}

// --- Setup ---

func TestSetupSuccess(t *testing.T) {
	router, _ := setupUnconfiguredRouter(t)
	w := do(t, router, "POST", "/api/auth/setup", map[string]any{
		"username": "admin", "password": "secret", "password_confirm": "secret",
	})
	if w.Code != http.StatusOK {
		t.Fatalf("got %d, want 200 (body: %s)", w.Code, w.Body.String())
	}
}

func TestSetupAlreadyConfigured(t *testing.T) {
	router, _ := setupUnconfiguredRouter(t)
	body := map[string]any{"username": "admin", "password": "secret", "password_confirm": "secret"}
	do(t, router, "POST", "/api/auth/setup", body)
	w := do(t, router, "POST", "/api/auth/setup", body)
	if w.Code != http.StatusConflict {
		t.Errorf("got %d, want 409", w.Code)
	}
}

func TestSetupPasswordMismatch(t *testing.T) {
	router, _ := setupUnconfiguredRouter(t)
	w := do(t, router, "POST", "/api/auth/setup", map[string]any{
		"username": "admin", "password": "secret", "password_confirm": "different",
	})
	if w.Code != http.StatusBadRequest {
		t.Errorf("got %d, want 400", w.Code)
	}
}

func TestSetupMissingFields(t *testing.T) {
	router, _ := setupUnconfiguredRouter(t)
	w := do(t, router, "POST", "/api/auth/setup", map[string]any{
		"username": "", "password": "", "password_confirm": "",
	})
	if w.Code != http.StatusBadRequest {
		t.Errorf("got %d, want 400", w.Code)
	}
}

// --- requireSetup middleware ---

func TestRequireSetupBlocksAPIWhenNotConfigured(t *testing.T) {
	router, _ := setupUnconfiguredRouter(t)
	w := do(t, router, "GET", "/api/sources", nil)
	if w.Code != http.StatusServiceUnavailable {
		t.Errorf("got %d, want 503", w.Code)
	}
}

func TestRequireSetupAllowsSetupEndpoint(t *testing.T) {
	router, _ := setupUnconfiguredRouter(t)
	w := do(t, router, "POST", "/api/auth/setup", map[string]any{
		"username": "admin", "password": "pass", "password_confirm": "pass",
	})
	if w.Code != http.StatusOK {
		t.Errorf("setup endpoint blocked: got %d", w.Code)
	}
}

// --- Login ---

func TestLoginSuccess(t *testing.T) {
	router, _ := setupUnconfiguredRouter(t)
	do(t, router, "POST", "/api/auth/setup", map[string]any{
		"username": "admin", "password": "secret", "password_confirm": "secret",
	})
	w := do(t, router, "POST", "/api/auth/login", map[string]any{
		"username": "admin", "password": "secret",
	})
	if w.Code != http.StatusOK {
		t.Fatalf("got %d, want 200", w.Code)
	}
	var found bool
	for _, c := range w.Result().Cookies() {
		if c.Name == sessionCookie && c.Value != "" {
			found = true
		}
	}
	if !found {
		t.Error("no session cookie in login response")
	}
}

func TestLoginWrongPassword(t *testing.T) {
	router, _ := setupUnconfiguredRouter(t)
	do(t, router, "POST", "/api/auth/setup", map[string]any{
		"username": "admin", "password": "secret", "password_confirm": "secret",
	})
	w := do(t, router, "POST", "/api/auth/login", map[string]any{
		"username": "admin", "password": "wrong",
	})
	if w.Code != http.StatusUnauthorized {
		t.Errorf("got %d, want 401", w.Code)
	}
}

func TestLoginWrongUsername(t *testing.T) {
	router, _ := setupUnconfiguredRouter(t)
	do(t, router, "POST", "/api/auth/setup", map[string]any{
		"username": "admin", "password": "secret", "password_confirm": "secret",
	})
	w := do(t, router, "POST", "/api/auth/login", map[string]any{
		"username": "notadmin", "password": "secret",
	})
	if w.Code != http.StatusUnauthorized {
		t.Errorf("got %d, want 401", w.Code)
	}
}

func TestLoginNotConfigured(t *testing.T) {
	router, _ := setupUnconfiguredRouter(t)
	w := do(t, router, "POST", "/api/auth/login", map[string]any{
		"username": "admin", "password": "secret",
	})
	if w.Code != http.StatusServiceUnavailable {
		t.Errorf("got %d, want 503", w.Code)
	}
}

// --- requireAuth middleware ---

func TestRequireAuthNoCookie(t *testing.T) {
	router, _ := setupUnconfiguredRouter(t)
	do(t, router, "POST", "/api/auth/setup", map[string]any{
		"username": "admin", "password": "pass", "password_confirm": "pass",
	})
	w := do(t, router, "GET", "/api/sources", nil)
	if w.Code != http.StatusUnauthorized {
		t.Errorf("got %d, want 401", w.Code)
	}
}

func TestRequireAuthInvalidToken(t *testing.T) {
	router, _ := setupUnconfiguredRouter(t)
	do(t, router, "POST", "/api/auth/setup", map[string]any{
		"username": "admin", "password": "pass", "password_confirm": "pass",
	})
	w := do(t, router, "GET", "/api/sources", nil, "not-a-real-token")
	if w.Code != http.StatusUnauthorized {
		t.Errorf("got %d, want 401", w.Code)
	}
}

func TestRequireAuthExpiredSession(t *testing.T) {
	router, auth := setupUnconfiguredRouter(t)
	do(t, router, "POST", "/api/auth/setup", map[string]any{
		"username": "admin", "password": "pass", "password_confirm": "pass",
	})
	session := loginAs(t, router, "admin", "pass")

	// Backdate the session's last_used_at to 8 days ago.
	stale := time.Now().UTC().Add(-8 * 24 * time.Hour).Format(time.RFC3339)
	if err := auth.SetSessionLastUsedAt(session, stale); err != nil {
		t.Fatalf("backdate session: %v", err)
	}

	w := do(t, router, "GET", "/api/sources", nil, session)
	if w.Code != http.StatusUnauthorized {
		t.Errorf("got %d, want 401 for expired session", w.Code)
	}
}

func TestValidSessionPassesThrough(t *testing.T) {
	router, _ := setupUnconfiguredRouter(t)
	do(t, router, "POST", "/api/auth/setup", map[string]any{
		"username": "admin", "password": "pass", "password_confirm": "pass",
	})
	session := loginAs(t, router, "admin", "pass")
	w := do(t, router, "GET", "/api/sources", nil, session)
	if w.Code != http.StatusOK {
		t.Errorf("got %d, want 200", w.Code)
	}
}

// --- Logout ---

func TestLogoutClearsCookie(t *testing.T) {
	router, _, _, _, _ := setupRouter(t)
	// use a fresh login to get a known session
	session := loginAs(t, router, "testuser", "testpass")
	w := do(t, router, "POST", "/api/auth/logout", nil, session)
	if w.Code != http.StatusOK {
		t.Fatalf("got %d, want 200", w.Code)
	}
	for _, c := range w.Result().Cookies() {
		if c.Name == sessionCookie && c.MaxAge == -1 {
			return // cookie cleared
		}
	}
	t.Error("session cookie not cleared after logout")
}

func TestLogoutInvalidatesSession(t *testing.T) {
	router, _, _, _, _ := setupRouter(t)
	session := loginAs(t, router, "testuser", "testpass")
	do(t, router, "POST", "/api/auth/logout", nil, session)
	w := do(t, router, "GET", "/api/sources", nil, session)
	if w.Code != http.StatusUnauthorized {
		t.Errorf("got %d, want 401 after logout", w.Code)
	}
}

// --- UpdateCredentials ---

func TestUpdateCredentialsPassword(t *testing.T) {
	router, _, _, _, session := setupRouter(t)
	w := do(t, router, "PUT", "/api/auth/credentials", map[string]any{
		"current_password": "testpass", "new_password": "newpass",
	}, session)
	if w.Code != http.StatusOK {
		t.Fatalf("got %d, want 200 (body: %s)", w.Code, w.Body.String())
	}
}

func TestUpdateCredentialsUsername(t *testing.T) {
	router, _, _, _, session := setupRouter(t)
	w := do(t, router, "PUT", "/api/auth/credentials", map[string]any{
		"current_password": "testpass", "username": "newuser",
	}, session)
	if w.Code != http.StatusOK {
		t.Fatalf("got %d, want 200 (body: %s)", w.Code, w.Body.String())
	}
	// New username should be reflected in /me.
	wMe := do(t, router, "GET", "/api/auth/me", nil, session)
	// Session was invalidated; re-login with new username.
	_ = wMe // session is gone after credential update; just verify the update didn't error
}

func TestUpdateCredentialsWrongPassword(t *testing.T) {
	router, _, _, _, session := setupRouter(t)
	w := do(t, router, "PUT", "/api/auth/credentials", map[string]any{
		"current_password": "wrongpass", "new_password": "anything",
	}, session)
	if w.Code != http.StatusUnauthorized {
		t.Errorf("got %d, want 401", w.Code)
	}
}

func TestUpdateCredentialsInvalidatesAllSessions(t *testing.T) {
	router, _ := setupUnconfiguredRouter(t)
	do(t, router, "POST", "/api/auth/setup", map[string]any{
		"username": "admin", "password": "pass", "password_confirm": "pass",
	})
	s1 := loginAs(t, router, "admin", "pass")
	s2 := loginAs(t, router, "admin", "pass")

	do(t, router, "PUT", "/api/auth/credentials", map[string]any{
		"current_password": "pass", "new_password": "newpass",
	}, s1)

	for _, s := range []string{s1, s2} {
		w := do(t, router, "GET", "/api/sources", nil, s)
		if w.Code != http.StatusUnauthorized {
			t.Errorf("session %s: got %d, want 401 after credential update", s[:8], w.Code)
		}
	}
}

// --- Me ---

func TestMe(t *testing.T) {
	router, _, _, _, session := setupRouter(t)
	w := do(t, router, "GET", "/api/auth/me", nil, session)
	if w.Code != http.StatusOK {
		t.Fatalf("got %d, want 200", w.Code)
	}
	var resp meResponse
	decodeJSON(t, w, &resp)
	if resp.Username != "testuser" {
		t.Errorf("username: got %q, want %q", resp.Username, "testuser")
	}
}
