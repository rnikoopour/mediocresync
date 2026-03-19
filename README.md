# go-ftpes

A web application for scheduling and monitoring incremental file copies from FTPES (FTP over Explicit TLS) servers. Configure connections and sync jobs through a browser UI; watch per-file progress live as transfers run.

## Requirements

- Go 1.25+
- Node.js 25.8.0+ (for building the frontend)

## Configuration

All configuration is via environment variables:

| Variable         | Required | Default      | Description |
|------------------|----------|--------------|-------------|
| `ENCRYPTION_KEY` | Yes      | —            | 32-byte hex string used to encrypt stored passwords |
| `LISTEN_ADDR`    | No       | `:8080`      | Address and port the server listens on |
| `DB_PATH`        | No       | `./mediocresync.db` | Path to the SQLite database file |
| `DEV_MODE`       | No       | `false`      | Enables CORS headers for local frontend development |

Generate an encryption key:

```sh
openssl rand -hex 32
```

> **Warning:** If you change `ENCRYPTION_KEY` after connections have been saved, the stored passwords will be unreadable and connections will need to be re-entered.

## Building

```sh
make build
```

This runs `npm run build` in `web/` (outputting to `ui/dist/`), then compiles the Go binary with the React app embedded. The result is a single binary at `bin/go-ftpes`.

## Running

```sh
export ENCRYPTION_KEY=<your-32-byte-hex-key>
./bin/go-ftpes
```

Open `http://localhost:8080` in your browser.

## Development

Start the Go server and Vite dev server concurrently:

```sh
export ENCRYPTION_KEY=<your-32-byte-hex-key>
export DEV_MODE=true
make dev
```

The React app runs at `http://localhost:5173` with `/api/*` proxied to the Go server at `:8080`.

## Testing

```sh
make test
```

## How it works

- **Connections** store FTPES server credentials (passwords encrypted with AES-256-GCM at rest).
- **Sync jobs** define what to copy, where to put it, and how often to run.
- On each run, files are compared by size and modification time against previously copied state. Only new or changed files are downloaded.
- In-progress files are written to `<local-dest>/.go-ftpes/<filename>` and atomically moved to their final path on success, so partial downloads never appear at the destination.
- Live transfer progress is streamed to the browser via Server-Sent Events.
- The scheduler runs inside the server process — no external queue or cron needed. If a job's interval fires while a run is still active, that interval is skipped.
