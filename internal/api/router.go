package api

import (
	"context"
	"io/fs"
	"log/slog"
	"net/http"

	"github.com/go-chi/chi/v5"
	chiMiddleware "github.com/go-chi/chi/v5/middleware"
	"github.com/rnikoopour/mediocresync/internal/db"
	internalsync "github.com/rnikoopour/mediocresync/internal/sync"
	"github.com/rnikoopour/mediocresync/internal/sse"
)

func NewRouter(
	appCtx context.Context,
	auth *db.AuthRepository,
	connections *db.ConnectionRepository,
	jobs *db.JobRepository,
	runs *db.RunRepository,
	transfers *db.TransferRepository,
	fileState *db.FileStateRepository,
	engine *internalsync.Engine,
	broker *sse.Broker,
	encKey []byte,
	devMode bool,
	logLevel *slog.LevelVar,
	staticFiles fs.FS,
) http.Handler {
	r := chi.NewRouter()

	r.Use(chiMiddleware.Recoverer)
	r.Use(requestID)
	r.Use(requestLogger)
	if devMode {
		r.Use(corsHeaders)
	}
	r.Use(requireSetup(auth))

	conns := &connectionsHandler{repo: connections, encKey: encKey}
	jobsH := &jobsHandler{repo: jobs, runs: runs, fileState: fileState, engine: engine, broker: broker, appCtx: appCtx}
	runsH := &runsHandler{runs: runs, transfers: transfers, broker: broker, appCtx: appCtx}
	authH := &authHandler{repo: auth}
	settingsH := &settingsHandler{logLevel: logLevel}

	r.Route("/api", func(r chi.Router) {
		r.Use(chiMiddleware.SetHeader("Content-Type", "application/json"))

		// Unauthenticated auth endpoints.
		r.Route("/auth", func(r chi.Router) {
			r.Post("/setup", authH.setup)
			r.Post("/login", authH.login)
		})

		// All other API routes require a valid session.
		r.Group(func(r chi.Router) {
			r.Use(requireAuth(auth))

			r.Post("/auth/logout", authH.logout)
			r.Put("/auth/credentials", authH.updateCredentials)
			r.Get("/auth/me", authH.me)

			r.Route("/connections", func(r chi.Router) {
				r.Get("/", conns.list)
				r.Post("/", conns.create)
				r.Post("/test", conns.testDirect)
				r.Get("/{id}", conns.get)
				r.Put("/{id}", conns.update)
				r.Delete("/{id}", conns.delete)
				r.Post("/{id}/test", conns.test)
				r.Get("/{id}/browse", conns.browse)
			})

			r.Route("/jobs", func(r chi.Router) {
				r.Get("/", jobsH.list)
				r.Post("/", jobsH.create)
				r.Get("/{id}", jobsH.get)
				r.Put("/{id}", jobsH.update)
				r.Delete("/{id}", jobsH.delete)
				r.Post("/{id}/run", jobsH.triggerRun)
				r.Post("/{id}/planthenrun", jobsH.planThenRun)
				r.Delete("/{id}/run", jobsH.cancelRun)
				r.Post("/{id}/plan", jobsH.planStart)
				r.Delete("/{id}/plan", jobsH.planDismiss)
				r.Get("/{id}/plan/events", jobsH.planEvents)
				r.Get("/{id}/events", jobsH.jobEvents)
				r.Put("/{id}/files", jobsH.putFileState)
				r.Delete("/{id}/files", jobsH.deleteFileState)
				r.Get("/{id}/runs", runsH.listByJob)
			})

			r.Get("/browse/local", localBrowse)

			r.Route("/settings", func(r chi.Router) {
				r.Get("/", settingsH.get)
				r.Put("/", settingsH.update)
			})

			r.Route("/runs", func(r chi.Router) {
				r.Get("/{id}", runsH.get)
				r.Get("/{id}/progress", runsH.progress)
			})
		})
	})

	// Serve the React SPA — unmatched paths fall back to index.html.
	r.Handle("/*", spaHandler(staticFiles))

	return r
}

// spaHandler serves static files and falls back to index.html for any path
// that doesn't match a real file, enabling client-side routing.
func spaHandler(files fs.FS) http.Handler {
	fileServer := http.FileServer(http.FS(files))
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, err := fs.Stat(files, r.URL.Path[1:]) // strip leading /
		if err != nil {
			// Not a real file — serve index.html for the SPA router.
			r.URL.Path = "/"
		}
		fileServer.ServeHTTP(w, r)
	})
}
