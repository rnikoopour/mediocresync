# go-ftpes Implementation Plan

## Status Legend
- [ ] Not started
- [~] In progress
- [x] Complete

---

## Phase 0 — Project Scaffolding
- [x] `go.mod` — module name, Go 1.22+, declare dependencies (`chi`, `modernc.org/sqlite`, `jlaffaye/ftp`)
- [x] Directory skeleton: `cmd/server/`, `internal/{config,crypto,db,ftpes,scheduler,sync,api,sse}/`, `web/`
- [x] `Makefile` with targets: `build` (React then Go), `dev` (concurrent Vite + Go), `test`, `lint`
- [x] `.gitignore` (`web/node_modules/`, `web/dist/`, `*.db`, `.env`)

## Phase 1 — Configuration & Secrets
- [x] `internal/config/config.go` — `Config` struct, `Load()` reads env vars (`LISTEN_ADDR`, `DB_PATH`, `ENCRYPTION_KEY`)
- [x] `internal/crypto/crypto.go` — AES-256-GCM `Encrypt` / `Decrypt`; key validated as 32 bytes

## Phase 2 — Database Layer
- [ ] `internal/db/schema.go` — DDL for 5 tables: `connections`, `sync_jobs`, `runs`, `transfers`, `file_state`
- [ ] `internal/db/migrate.go` — `Migrate(db)` runs `CREATE TABLE IF NOT EXISTS` at startup
- [ ] `internal/db/db.go` — `Open(path)` opens SQLite, sets WAL + foreign keys, calls `Migrate`
- [ ] `internal/db/connections.go` — `ConnectionRepository`: `Create`, `List`, `Get`, `Update`, `Delete`
- [ ] `internal/db/jobs.go` — `JobRepository`: `Create`, `List`, `Get`, `Update`, `Delete`, `ListEnabled`
- [ ] `internal/db/runs.go` — `RunRepository`: `Create`, `Get`, `ListByJob`, `UpdateStatus`, `UpdateCounts`
- [ ] `internal/db/transfers.go` — `TransferRepository`: `Create`, `UpdateProgress`, `UpdateStatus`, `ListByRun`
- [ ] `internal/db/filestate.go` — `FileStateRepository`: `Upsert`, `Get`, `DeleteByJob`

## Phase 3 — FTPES Client
- [ ] `internal/ftpes/dial.go` — `Dial(host, port, skipVerify)` with explicit TLS (`AUTH TLS`)
- [ ] `internal/ftpes/walk.go` — recursive `LIST` traversal, returns flat `[]RemoteFile`
- [ ] `internal/ftpes/client.go` — `Client` struct with `Login`, `Walk`, `Download`, `Close`; backed by interface for testability

## Phase 4 — Sync Engine
- [ ] `internal/sync/fingerprint.go` — `Matches(state, remoteFile)` with 1-second mtime tolerance
- [ ] `internal/sync/staging.go` — `stagingDir`, `stagingPath`, `finalPath`, `atomicMove`
- [ ] `internal/sync/progress.go` — `progressReader` wrapping `io.Reader`, rate-limited callbacks (~250ms)
- [ ] `internal/sync/engine.go` — `Engine.RunJob(ctx, jobID)`: dial → walk → diff → stage → stream → rename → upsert state

## Phase 5 — SSE Broker
- [ ] `internal/sse/event.go` — `Event` struct (runID, transferID, path, size, bytes, percent, speed, status, error)
- [ ] `internal/sse/broker.go` — `Broker` with `Subscribe(runID)`, `Publish(runID, event)`, `Close(runID)`; non-blocking publish via buffered channels

## Phase 6 — Scheduler
- [ ] `internal/scheduler/scheduler.go` — `Scheduler` with 1-minute tick, `active` map for overlap prevention, `Start`, `Stop`, `TriggerNow`

## Phase 7 — REST API Handlers
- [ ] `internal/api/response.go` — shared helpers: `writeJSON`, `writeError`, `readJSON`
- [ ] `internal/api/middleware.go` — request ID, structured logging, CORS (dev mode)
- [ ] `internal/api/connections.go` — `List`, `Create`, `Get`, `Update`, `Delete`, `Test` handlers
- [ ] `internal/api/jobs.go` — `List`, `Create`, `Get`, `Update`, `Delete`, `TriggerRun` handlers
- [ ] `internal/api/runs.go` — `List`, `Get`, `GetProgress` (SSE) handlers
- [ ] `internal/api/router.go` — chi router, mount all handlers, serve embedded React with SPA fallback

## Phase 8 — Entry Point & Embedding
- [ ] `cmd/server/embed.go` — `//go:embed ../../web/dist` var
- [ ] `cmd/server/main.go` — wire all subsystems, start scheduler, graceful shutdown on SIGINT/SIGTERM

## Phase 9 — React Frontend
- [ ] `web/` — Vite + React + TypeScript scaffold (`npm create vite`)
- [ ] `web/src/api/types.ts` — TypeScript interfaces matching Go API response shapes
- [ ] `web/src/api/client.ts` — typed fetch wrappers for all REST endpoints
- [ ] `web/src/hooks/useSSE.ts` — `EventSource` hook, merges events into `Map<transferID, Event>`
- [ ] `web/src/components/` — `Layout`, `StatusBadge`, `ProgressBar`
- [ ] `web/src/pages/ConnectionsPage.tsx` — list, add/edit modal, delete, test button
- [ ] `web/src/pages/JobsPage.tsx` — list, add/edit modal, delete, last run + next run display
- [ ] `web/src/pages/JobDetailPage.tsx` — runs list, manual trigger button
- [ ] `web/src/pages/RunDetailPage.tsx` — transfer table with live SSE progress for active runs
- [ ] `web/src/App.tsx` — route definitions
- [ ] `vite.config.ts` — dev proxy `/api/*` → `http://localhost:8080`

## Phase 10 — Testing
- [ ] `internal/crypto/crypto_test.go` — encrypt/decrypt round-trip, wrong key error
- [ ] `internal/db/db_test.go` — all repo methods against `:memory:` SQLite
- [ ] `internal/sync/staging_test.go` — `finalPath` edge cases (trailing slash, root remote path)
- [ ] `internal/sync/fingerprint_test.go` — mtime tolerance boundary cases
- [ ] `internal/sse/broker_test.go` — subscribe/publish/unsubscribe/close, non-blocking publish under full buffer
- [ ] `internal/api/handlers_test.go` — `httptest` + mocked repos, verify status codes and JSON shapes

## Phase 11 — Packaging
- [ ] `Dockerfile` — multi-stage: `node:20-alpine` (React build) → `golang:1.22-alpine` (Go build) → `distroless/static`
- [ ] `README.md` — env vars, key generation (`openssl rand -hex 32`), dev setup, production deploy

---

## Key Technical Decisions

| Area | Decision |
|---|---|
| SQLite driver | `modernc.org/sqlite` — pure Go, no CGO, single binary |
| WAL mode | Enabled at open — concurrent SSE reads during active syncs |
| Password encryption | AES-256-GCM, key from `ENCRYPTION_KEY` env var |
| Staging dir | Always under `<local-dest>/.go-ftpes/<filename>` — same partition guarantees atomic rename |
| SSE fan-out | Buffered channels, non-blocking publish — slow clients drop events, never stall sync |
| Scheduler overlap | In-memory `active` map — skip, never queue |
| Scheduler tick | 1-minute granularity — matches minimum schedule unit (N minutes) |
| API/DB structs | Separate types with explicit mappers — API and schema evolve independently |
| React data | `@tanstack/react-query` — cache invalidation and background refetch |
| Binary embedding | `//go:embed web/dist` — single binary + single `.db` file deployment |

---

## Risk Areas

- **`sync/engine.go`** — highest complexity: staging, progress, state updates, and error handling all converge
- **`sse/broker.go`** — concurrent channel lifecycle; non-blocking publish must never stall the sync goroutine
- **FTPES explicit TLS** — verify `jlaffaye/ftp` option name for `AUTH TLS`; test against a real server early
- **`os.Rename` atomicity** — safe only within same partition; staging dir placement enforces this
- **SSE behind reverse proxy** — response buffering must be disabled; document for operators
