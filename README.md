# MediocreSync

A web application for scheduling and monitoring sync jobs from FTPES (FTP over Explicit TLS) servers and Git repositories. Configure sources and jobs through a browser UI; watch per-file progress and live logs as transfers run.

## Requirements

- Go 1.25+
- Node.js 25.8.1+ (for building the frontend)

## Configuration

All configuration is via environment variables:

| Variable                      | Required | Default      | Description |
|-------------------------------|----------|--------------|-------------|
| `MEDIOCRESYNC_LISTEN_ADDR`    | No       | `:8080`      | Address and port the server listens on |
| `MEDIOCRESYNC_DB_PATH`        | No       | `~/.mediocresync/mediocresync.db` | Path to the SQLite database file |
| `MEDIOCRESYNC_LOG_FILE`       | No       | `~/.mediocresync/mediocresync.log` | Path to the log file (rotated automatically) |
| `MEDIOCRESYNC_LOG_LEVEL`      | No       | `info`       | Initial log verbosity: `debug`, `info`, `warn`, or `error`. Can be changed at runtime via the Settings page. |
| `MEDIOCRESYNC_DEV_MODE`       | No       | `false`      | Enables CORS headers for local frontend development |

On first startup the server generates a random AES-256 encryption key and stores it in the database. Stored FTPES passwords are encrypted with that key. Deleting or replacing the database will make existing credentials unreadable.

## Deploying with Docker

Images are published to `ghcr.io/rnikoopour/mediocresync` on each version tag. Replace `<version>` with the desired release (e.g. `v1.2.0`):

```sh
docker run -d \
  --name mediocresync \
  --restart unless-stopped \
  -p 8080:8080 \
  -v /path/to/data:/data \
  ghcr.io/rnikoopour/mediocresync:<version>
```

The `/data` volume holds the SQLite database and log file. To override any environment variable:

```sh
docker run -d \
  --name mediocresync \
  --restart unless-stopped \
  -p 8080:8080 \
  -v /path/to/data:/data \
  -e MEDIOCRESYNC_LISTEN_ADDR=:9000 \
  ghcr.io/rnikoopour/mediocresync:<version>
```

Open `http://localhost:8080` in your browser.

## Building

```sh
make build
```

This runs `npm run build` in `web/` (outputting to `ui/dist/`), then compiles the Go binary with the React app embedded. The result is a single binary at `bin/mediocresync`.

## Running

```sh
./bin/mediocresync
```

Open `http://localhost:8080` in your browser.

## Development

Start the Go server and Vite dev server concurrently:

```sh
export MEDIOCRESYNC_DEV_MODE=true
make run-dev
```

The React app runs at `http://localhost:5173` with `/api/*` proxied to the Go server at `:8080`.

## Testing

```sh
make test
```

## How it works

- **Sources** store connection details for FTPES servers or Git repositories. Credentials are encrypted with AES-256-GCM at rest.
- **Sync jobs** define what to copy, where to put it, and how often to run.
- The scheduler runs inside the server process — no external queue or cron needed. Jobs fire at clock-aligned boundaries (e.g. every 60 min → 00:00, 01:00, 02:00). If a slot fires while a run is still active, that slot is skipped.
- Live transfer progress and logs are streamed to the browser via Server-Sent Events.

### FTPES jobs

- Files are compared by size and modification time against previously synced state. Only new or changed files are downloaded.
- In-progress files are written to `<local-dest>/.mediocresync/<filename>` and atomically moved to their final path on success, so partial downloads never appear at the destination.

### Git jobs

- Each job holds a list of Git repository URLs (HTTPS or SSH). Repositories are cloned into `<local-dest>/<host>/<org>/<repo>`.
- On subsequent runs the tracked branch is fetched and the working tree is hard-reset to the remote tip, so local modifications never block a sync.
- Auth options: anonymous, personal access token (HTTP Basic), or SSH private key.
- A **plan** phase fetches the remote to detect upstream changes and records the previous and current commit hashes before applying the update.
