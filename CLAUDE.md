# Tests
- ALWAYS run `make test` after making any code changes. Do not commit or consider a task complete until all tests pass.

# Real-time Updates
- Use SSE for all real-time UI updates. Do not add polling (`refetchInterval`).
- SSE invalidates React Query cache when events arrive; queries fetch the final state once.

# Architecture

## Go Backend (`internal/`)

### Sync Engine (`internal/sync/`)
- `engine.go` — `Engine` struct; `PlanJobStream`, `runWithPlan`, `makeSource`, `newSource`
- `source.go` — `Source` interface, `PlanInput`, `SyncInput`, `TransferEvent`, `PlanOutput`, `PlanFile`
- `ftpes_source.go` — `FTPESSource`; lazy-decrypt `getPassword` closure; no plaintext in struct
- `git_source.go` — `GitSource`; lazy-decrypt `buildAuth` closure; no plaintext in struct
- `engine_test.go` — `mockSource`, `openTestEngine`, count-invariant tests

**Key invariants:**
- Sources have no db fields — all persistence via engine's `onEvent` closure
- Engine pre-processes `skip`/`error` plan entries; `Sync` only sees `copy` entries
- `failed` seeded with `initialFailed` (plan-time errors) so counts are always accurate
- `Sync` returns nil for transfer errors; only hard setup failures return non-nil
- `onEvent` is called concurrently — mutex protects `copied`/`failed` counters

### Other packages
- `internal/db/` — SQLite repositories (sources, jobs, runs, transfers, sync_state, git_repos)
- `internal/gitsource/` — git clone/fetch logic; `EnumerateWithAuth` for pre-built auth
- `internal/ftpes/` — FTP-ES client; `ListDir`, `DownloadFile`
- `internal/sse/` — `Broker`; `broker.Publish(channel, event)`
- `internal/api/` — HTTP handlers; thin layer over db + engine
- `internal/crypto/` — `Encrypt`/`Decrypt` for stored credentials

## Frontend (`web/src/`)

### Pages
- `pages/JobDetailPage.tsx` — job header, plan bar, run history list (imports RunRow, PlanTreeView)
- `pages/RunDetailPage.tsx` — full run detail with transfer tree

### Key Components
- `components/RunRow.tsx` — `RunRow`, `GitRunView`, `formatDuration`, `useElapsed`
- `components/PlanTreeView.tsx` — `PlanTreeView`, `GitPlanView`, `TreeFile` type
- `components/RunTree.tsx` — `RunTreeView`, `RunTabBar`, `formatBytes`, `formatSpeed`
- `components/StatusBadge.tsx` — color-coded status pill for all statuses
- `components/JobFormModal.tsx` — create/edit job form

### Hooks & Context
- `hooks/useSSE.ts` — `useSSE(runID | null)` → `{ events, runStatus }`; auto-reconnects
- `context/PlanContext.tsx` — plan state per job; `runPlan`, `subscribePlan`, `dismissPlan`
- `hooks/useLocalStorageBool.ts` — persisted boolean toggle
