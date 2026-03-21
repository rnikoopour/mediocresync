package api

import (
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/rnikoopour/mediocresync/internal/db"
)

type responseRecorder struct {
	http.ResponseWriter
	status int
}

func (r *responseRecorder) WriteHeader(status int) {
	r.status = status
	r.ResponseWriter.WriteHeader(status)
}

func (r *responseRecorder) Flush() {
	if f, ok := r.ResponseWriter.(http.Flusher); ok {
		f.Flush()
	}
}

func requestLogger(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		rec := &responseRecorder{ResponseWriter: w, status: http.StatusOK}
		next.ServeHTTP(rec, r)
		slog.Debug("request",
			"method", r.Method,
			"path", r.URL.Path,
			"status", rec.status,
			"duration_ms", time.Since(start).Milliseconds(),
		)
	})
}

func requestID(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id := uuid.New().String()
		w.Header().Set("X-Request-ID", id)
		next.ServeHTTP(w, r)
	})
}

// requireSetup redirects or rejects requests when credentials have not yet
// been configured. Auth endpoints and the /setup and /login UI routes are
// always allowed through so the setup flow can complete.
func requireSetup(repo *db.AuthRepository) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Always allow auth endpoints and the setup/login UI routes.
			if strings.HasPrefix(r.URL.Path, "/api/auth/") ||
				r.URL.Path == "/setup" || r.URL.Path == "/login" {
				next.ServeHTTP(w, r)
				return
			}

			_, _, configured, err := repo.GetCredentials()
			if err != nil {
				writeError(w, http.StatusInternalServerError, "internal error")
				return
			}
			if configured {
				next.ServeHTTP(w, r)
				return
			}

			if strings.HasPrefix(r.URL.Path, "/api/") {
				writeError(w, http.StatusServiceUnavailable, "setup_required")
				return
			}
			// Browser navigations to UI routes — redirect to setup.
			if strings.Contains(r.Header.Get("Accept"), "text/html") {
				http.Redirect(w, r, "/setup", http.StatusFound)
				return
			}
			// Static assets (JS, CSS, etc.) pass through so the setup page loads.
			next.ServeHTTP(w, r)
		})
	}
}

// requireAuth rejects requests that do not carry a valid, active session.
func requireAuth(repo *db.AuthRepository) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			cookie, err := r.Cookie(sessionCookie)
			if err != nil {
				rejectUnauthenticated(w, r)
				return
			}

			lastUsed, found, err := repo.LookupSession(cookie.Value)
			if err != nil || !found {
				rejectUnauthenticated(w, r)
				return
			}

			if time.Since(lastUsed) > 7*24*time.Hour {
				_ = repo.DeleteSession(cookie.Value)
				rejectUnauthenticated(w, r)
				return
			}

			_ = repo.TouchSession(cookie.Value)
			next.ServeHTTP(w, r)
		})
	}
}

func rejectUnauthenticated(w http.ResponseWriter, r *http.Request) {
	if strings.HasPrefix(r.URL.Path, "/api/") {
		writeError(w, http.StatusUnauthorized, "unauthenticated")
		return
	}
	http.Redirect(w, r, "/login", http.StatusFound)
}

func corsHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}
