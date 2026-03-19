package api

import (
	"io/fs"
	"net/http"

	"github.com/go-chi/chi/v5"
	chiMiddleware "github.com/go-chi/chi/v5/middleware"
	"github.com/rnikoopour/go-ftpes/internal/db"
	internalsync "github.com/rnikoopour/go-ftpes/internal/sync"
	"github.com/rnikoopour/go-ftpes/internal/sse"
)

func NewRouter(
	connections *db.ConnectionRepository,
	jobs *db.JobRepository,
	runs *db.RunRepository,
	transfers *db.TransferRepository,
	fileState *db.FileStateRepository,
	engine *internalsync.Engine,
	broker *sse.Broker,
	encKey []byte,
	devMode bool,
	staticFiles fs.FS,
) http.Handler {
	r := chi.NewRouter()

	r.Use(chiMiddleware.Recoverer)
	r.Use(requestID)
	r.Use(requestLogger)
	if devMode {
		r.Use(corsHeaders)
	}

	conns := &connectionsHandler{repo: connections, encKey: encKey}
	jobsH := &jobsHandler{repo: jobs, fileState: fileState, engine: engine}
	runsH := &runsHandler{runs: runs, transfers: transfers, broker: broker}

	r.Route("/api", func(r chi.Router) {
		r.Use(chiMiddleware.SetHeader("Content-Type", "application/json"))

		r.Route("/connections", func(r chi.Router) {
			r.Get("/", conns.list)
			r.Post("/", conns.create)
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
			r.Post("/{id}/plan", jobsH.plan)
			r.Get("/{id}/runs", runsH.listByJob)
		})

		r.Route("/runs", func(r chi.Router) {
			r.Get("/{id}", runsH.get)
			r.Get("/{id}/progress", runsH.progress)
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
