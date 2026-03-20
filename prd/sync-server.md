# PRD: MediocreSync — FTPES Sync Web App

## Overview

A Go backend + React frontend web application that manages scheduled, incremental file copies from one or more FTPES (FTP over Explicit TLS) servers to local destinations. Users configure servers and sync jobs through a React UI, monitor live file transfer progress, and trigger runs on demand.

---

## Goals

- Provide a React web UI to manage FTPES server connections and sync jobs
- Run sync jobs on a configurable interval schedule
- Incrementally copy new or changed files, preserving remote directory structure
- Show live per-file progress during active sync runs
- Allow manual (on-demand) job triggers from the UI
- Track which files have already been copied and skip them on subsequent runs

## Non-Goals

- Uploading files to the FTPES server
- Supporting plain FTP or implicit FTPS
- Multi-user authentication / access control (single-user app)
- Cloud storage destinations (S3, GCS, etc.)

---

## Core Concepts

| Entity         | Description |
|----------------|-------------|
| **Connection** | FTPES server credentials and TLS settings |
| **Sync Job**   | Links a Connection to a remote path, local destination, and a schedule |
| **Run**        | A single execution of a sync job (scheduled or manual) |
| **Transfer**   | One file being copied within a run |

---

## React Frontend

### Views / Pages

| View                 | Description |
|----------------------|-------------|
| **Connections**      | List, add, edit, delete FTPES server connections. Test connectivity. |
| **Sync Jobs**        | List, add, edit, delete sync jobs. Show last run status and next scheduled run. |
| **Job Detail**       | Active and historical runs for a job; per-file transfer status and progress |
| **Run Detail**       | Full file list for a specific run with live progress when active |

### Live Progress

When a run is active, the Run Detail view streams live updates per file:
- Filename and remote path
- File size
- Bytes transferred + percentage complete
- Transfer speed
- Status: pending / in-progress / done / skipped / failed

Updates are delivered via Server-Sent Events (SSE) consumed by the React app.

---

## Connection Configuration

Fields per connection:
- Name (display label)
- Host
- Port (default: 21)
- Username
- Password (stored encrypted at rest)
- Skip TLS certificate verification (toggle)

---

## Sync Job Configuration

Fields per job:
- Name (display label)
- Connection (select from configured connections)
- Remote path (directory to copy from, default: `/`)
- Local destination path (absolute path on the server host)
- Schedule interval: every N minutes / hours / days
- Enabled toggle (pause without deleting)

---

## Sync Behavior

### Incremental Copy

- Recursively traverse the remote directory tree from the configured remote path
- For each remote file, compare against state (remote path + size + mtime)
- Skip files whose path and fingerprint match a previously successful transfer
- Re-download files whose size or mtime has changed
- Preserve the remote directory structure under the local destination:

  ```
  Remote: /exports/reports/2024/jan.csv
  Remote path config: /exports
  Local destination: /data/downloads
  Result: /data/downloads/reports/2024/jan.csv
  ```

### In-Progress File Staging

To prevent partial or corrupt files from appearing at the final destination:

1. Each file is first downloaded to a flat staging directory: `<local-destination>/.mediocresync/<filename>`
   - Example: `/data/downloads/.mediocresync/jan.csv`
2. On successful completion, the staged file is atomically moved (`os.Rename`) to its final destination path (with full directory structure)
3. On failure, the staged file is deleted; the final destination path is never written to

The `.mediocresync/` staging directory is created automatically and managed entirely by the app. Users should not write files into it.

### Transfer Safety

- Stream file contents directly to the staging path; do not buffer entire files in memory
- Create intermediate directories at the final destination as needed
- On failure: delete the staged file, record the error in the run log, continue with remaining files
- State is updated only after a successful rename to the final destination

### State Storage

- State stored in SQLite (embedded, no separate service required; single `.db` file on disk)
- Tracks per-job, per-file: remote path, size, mtime, copied_at timestamp

---

## Scheduling

- Each job defines a simple interval: every N minutes, hours, or days
- The scheduler runs within the Go server process
- If a job's scheduled interval fires while a run for that job is still active, the scheduled run is silently skipped — no queuing, no overlap
- Disabled jobs are not scheduled

---

## Run Lifecycle

```
scheduled / manual trigger
        ↓
    CONNECTING
        ↓
    RUNNING  ← per-file progress streamed via SSE
               files staged at <dest>/.mediocresync/<filename>
               then moved to <dest>/<relative-path> on success
        ↓
 COMPLETED | FAILED
```

Run record stores:
- Job ID
- Start time, end time
- Status (running / completed / failed)
- Counts: total files, copied, skipped, failed
- Per-transfer records (file path, size, bytes transferred, duration, status, error message)

---

## Technical Stack

| Concern             | Approach |
|---------------------|----------|
| Language            | Go 1.22+ |
| Backend router      | `net/http` stdlib + lightweight router (e.g. `chi`) |
| Frontend            | React (Vite build) |
| Real-time updates   | Server-Sent Events (SSE) |
| Database            | SQLite via `modernc.org/sqlite` (embedded, no external service) |
| FTP/FTPES           | `github.com/jlaffaye/ftp` or similar |
| TLS                 | `crypto/tls` stdlib |
| Scheduling          | In-process ticker/scheduler |
| Password storage    | Encrypted at rest (AES-GCM, key from env var or config) |

The Go binary serves the React build as static assets so a single binary + SQLite `.db` file is the full deployment.

---

## REST API (Go backend)

| Method | Path                        | Description |
|--------|-----------------------------|-------------|
| GET    | `/api/connections`          | List connections |
| POST   | `/api/connections`          | Create connection |
| GET    | `/api/connections/:id`      | Get connection |
| PUT    | `/api/connections/:id`      | Update connection |
| DELETE | `/api/connections/:id`      | Delete connection |
| POST   | `/api/connections/:id/test` | Test connectivity |
| GET    | `/api/jobs`                 | List sync jobs |
| POST   | `/api/jobs`                 | Create sync job |
| GET    | `/api/jobs/:id`             | Get sync job |
| PUT    | `/api/jobs/:id`             | Update sync job |
| DELETE | `/api/jobs/:id`             | Delete sync job |
| POST   | `/api/jobs/:id/run`         | Trigger manual run |
| GET    | `/api/jobs/:id/runs`        | List runs for job |
| GET    | `/api/runs/:id`             | Get run detail + transfers |
| GET    | `/api/runs/:id/progress`    | SSE stream of live per-file progress |

---

## Success Criteria

1. User can add an FTPES connection and verify connectivity via the React UI
2. User can create a sync job with an interval schedule and see it run automatically
3. Scheduled runs that overlap with an in-progress run for the same job are skipped
4. User can trigger a run manually and watch per-file progress update live
5. Re-running a job skips already-downloaded files; changed files are re-downloaded
6. Local directory structure mirrors the remote tree
7. In-progress files are staged at `<dest>/.mediocresync/<filename>` and only moved to their final path on success — no partial files at the destination
8. Failed transfers do not corrupt state
9. App survives restart: schedule resumes, state is preserved in SQLite
10. Single deployable artifact: one Go binary embedding the React build
