package db

// alterations are run after schema and are idempotent — duplicate column errors are ignored.
var alterations = []string{
	`ALTER TABLE sync_jobs ADD COLUMN include_filters TEXT NOT NULL DEFAULT '[]'`,
	`ALTER TABLE sync_jobs ADD COLUMN exclude_filters TEXT NOT NULL DEFAULT '[]'`,
	`ALTER TABLE connections ADD COLUMN enable_epsv INTEGER NOT NULL DEFAULT 0`,
	`ALTER TABLE runs ADD COLUMN total_size_bytes INTEGER NOT NULL DEFAULT 0`,
	`ALTER TABLE sync_jobs ADD COLUMN retry_attempts INTEGER NOT NULL DEFAULT 3`,
	`ALTER TABLE sync_jobs ADD COLUMN retry_delay_seconds INTEGER NOT NULL DEFAULT 2`,
}

var schema = []string{
	`CREATE TABLE IF NOT EXISTS _meta (
		key   TEXT PRIMARY KEY,
		value TEXT NOT NULL
	)`,

	`CREATE TABLE IF NOT EXISTS connections (
		id              TEXT PRIMARY KEY,
		name            TEXT NOT NULL,
		host            TEXT NOT NULL,
		port            INTEGER NOT NULL DEFAULT 21,
		username        TEXT NOT NULL,
		password        BLOB NOT NULL,
		skip_tls_verify INTEGER NOT NULL DEFAULT 0,
		created_at      TEXT NOT NULL,
		updated_at      TEXT NOT NULL
	)`,

	`CREATE TABLE IF NOT EXISTS sync_jobs (
		id              TEXT PRIMARY KEY,
		name            TEXT NOT NULL,
		connection_id   TEXT NOT NULL REFERENCES connections(id),
		remote_path     TEXT NOT NULL DEFAULT '/',
		local_dest      TEXT NOT NULL,
		interval_value  INTEGER NOT NULL,
		interval_unit   TEXT NOT NULL CHECK(interval_unit IN ('minutes','hours','days')),
		concurrency     INTEGER NOT NULL DEFAULT 1,
		enabled         INTEGER NOT NULL DEFAULT 1,
		created_at      TEXT NOT NULL,
		updated_at      TEXT NOT NULL
	)`,

	`CREATE TABLE IF NOT EXISTS runs (
		id            TEXT PRIMARY KEY,
		job_id        TEXT NOT NULL REFERENCES sync_jobs(id),
		status        TEXT NOT NULL CHECK(status IN ('running','completed','failed','canceled','server_stopped')),
		started_at    TEXT NOT NULL,
		finished_at   TEXT,
		total_files      INTEGER NOT NULL DEFAULT 0,
		copied_files     INTEGER NOT NULL DEFAULT 0,
		skipped_files    INTEGER NOT NULL DEFAULT 0,
		failed_files     INTEGER NOT NULL DEFAULT 0,
		total_size_bytes INTEGER NOT NULL DEFAULT 0
	)`,

	`CREATE TABLE IF NOT EXISTS transfers (
		id           TEXT PRIMARY KEY,
		run_id       TEXT NOT NULL REFERENCES runs(id),
		remote_path  TEXT NOT NULL,
		local_path   TEXT NOT NULL,
		size_bytes   INTEGER NOT NULL DEFAULT 0,
		bytes_xferred INTEGER NOT NULL DEFAULT 0,
		duration_ms  INTEGER,
		status       TEXT NOT NULL CHECK(status IN ('pending','in_progress','done','skipped','failed')),
		error_msg    TEXT,
		started_at   TEXT,
		finished_at  TEXT
	)`,

	`CREATE TABLE IF NOT EXISTS file_state (
		job_id      TEXT NOT NULL REFERENCES sync_jobs(id),
		remote_path TEXT NOT NULL,
		size_bytes  INTEGER NOT NULL,
		mtime       TEXT NOT NULL,
		copied_at   TEXT NOT NULL,
		PRIMARY KEY (job_id, remote_path)
	)`,
}
