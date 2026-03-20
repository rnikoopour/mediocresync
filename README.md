# MediocreSync

A web application for scheduling and monitoring incremental file copies from FTPES (FTP over Explicit TLS) servers. Configure connections and sync jobs through a browser UI; watch per-file progress live as transfers run.

## Requirements

- Go 1.25+
- Node.js 25.8.0+ (for building the frontend)

## Configuration

All configuration is via environment variables:

| Variable      | Required | Default      | Description |
|---------------|----------|--------------|-------------|
| `LISTEN_ADDR` | No       | `:8080`      | Address and port the server listens on |
| `DB_PATH`     | No       | `~/.mediocresync/mediocresync.db` | Path to the SQLite database file |
| `DEV_MODE`    | No       | `false`      | Enables CORS headers for local frontend development |

On first startup the server generates a random AES-256 encryption key and stores it in the database. Stored FTPES passwords are encrypted with that key. Deleting or replacing the database will make existing credentials unreadable.

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
export DEV_MODE=true
make run-dev
```

The React app runs at `http://localhost:5173` with `/api/*` proxied to the Go server at `:8080`.

## Testing

```sh
make test
```

## Deploying on Ubuntu

Download the latest `mediocresync` binary from the [GitHub releases page](https://github.com/rnikoopour/mediocresync/releases), then:

```sh
# Create a dedicated user
sudo useradd -r -s /bin/false mediocresync

# Install the binary
sudo cp mediocresync /usr/local/bin/mediocresync
sudo chmod +x /usr/local/bin/mediocresync

# Install and enable the systemd service
sudo cp mediocresync.service /etc/systemd/system/
sudo systemctl daemon-reload
sudo systemctl enable --now mediocresync
```

The service listens on port `5000` and stores its database at `/var/lib/mediocresync/mediocresync.db` (created automatically by systemd). To override, edit the `Environment=` lines in the unit file before enabling.

## How it works

- **Connections** store FTPES server credentials (passwords encrypted with AES-256-GCM at rest).
- **Sync jobs** define what to copy, where to put it, and how often to run.
- On each run, files are compared by size and modification time against previously copied state. Only new or changed files are downloaded.
- In-progress files are written to `<local-dest>/.mediocresync/<filename>` and atomically moved to their final path on success, so partial downloads never appear at the destination.
- Live transfer progress is streamed to the browser via Server-Sent Events.
- The scheduler runs inside the server process — no external queue or cron needed. Jobs fire at clock-aligned boundaries (e.g. every 60 min → 00:00, 01:00, 02:00). If a slot fires while a run is still active, that slot is skipped.
