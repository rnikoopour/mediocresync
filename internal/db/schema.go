package db

// alterations are run after schema and are idempotent — duplicate column name
// and missing table errors are both ignored (the latter occurs after versioned
// migrations drop tables that some alterations still reference, e.g. connections).
var alterations = []string{
	`ALTER TABLE sync_jobs ADD COLUMN include_filters TEXT NOT NULL DEFAULT '[]'`,
	`ALTER TABLE sync_jobs ADD COLUMN exclude_filters TEXT NOT NULL DEFAULT '[]'`,
	// Kept for databases that pre-date the sources migration; ignored once
	// the connections table has been dropped by the versioned migration.
	`ALTER TABLE connections ADD COLUMN enable_epsv INTEGER NOT NULL DEFAULT 0`,
	`ALTER TABLE runs ADD COLUMN total_size_bytes INTEGER NOT NULL DEFAULT 0`,
	`ALTER TABLE sync_jobs ADD COLUMN retry_attempts INTEGER NOT NULL DEFAULT 3`,
	`ALTER TABLE sync_jobs ADD COLUMN retry_delay_seconds INTEGER NOT NULL DEFAULT 2`,
	`ALTER TABLE sync_jobs ADD COLUMN include_path_filters TEXT NOT NULL DEFAULT '[]'`,
	`ALTER TABLE sync_jobs ADD COLUMN include_name_filters TEXT NOT NULL DEFAULT '[]'`,
	`ALTER TABLE sync_jobs ADD COLUMN exclude_path_filters TEXT NOT NULL DEFAULT '[]'`,
	`ALTER TABLE sync_jobs ADD COLUMN exclude_name_filters TEXT NOT NULL DEFAULT '[]'`,
	`ALTER TABLE sync_jobs ADD COLUMN run_retention_days INTEGER NOT NULL DEFAULT 0`,
	`ALTER TABLE runs ADD COLUMN error_msg TEXT`,
	`ALTER TABLE transfers ADD COLUMN previous_commit_hash TEXT`,
	`ALTER TABLE transfers ADD COLUMN current_commit_hash TEXT`,
}

var schema = []string{
	`CREATE TABLE IF NOT EXISTS _meta (
		key   TEXT PRIMARY KEY,
		value TEXT NOT NULL
	)`,

	// connections is kept so that the create_sources_table versioned migration
	// can copy from it on fresh databases. Dropped by drop_connections_table.
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

	`CREATE TABLE IF NOT EXISTS sources (
		id              TEXT PRIMARY KEY,
		name            TEXT NOT NULL,
		type            TEXT NOT NULL CHECK(type IN ('ftpes','git')),
		host            TEXT,
		port            INTEGER,
		username        TEXT,
		password        BLOB,
		skip_tls_verify INTEGER NOT NULL DEFAULT 0,
		enable_epsv     INTEGER NOT NULL DEFAULT 0,
		auth_type       TEXT CHECK(auth_type IN ('none','token','ssh_key')),
		auth_credential BLOB,
		created_at      TEXT NOT NULL,
		updated_at      TEXT NOT NULL
	)`,

	// sync_jobs is defined with connection_id so that pre-migration databases
	// and fresh databases both have the column available for the
	// sync_jobs_use_source_id versioned migration to copy from.
	// After that migration runs, the table is rebuilt with source_id.
	`CREATE TABLE IF NOT EXISTS sync_jobs (
		id                   TEXT PRIMARY KEY,
		name                 TEXT NOT NULL,
		connection_id        TEXT NOT NULL REFERENCES connections(id),
		remote_path          TEXT NOT NULL DEFAULT '/',
		local_dest           TEXT NOT NULL,
		interval_value       INTEGER NOT NULL,
		interval_unit        TEXT NOT NULL CHECK(interval_unit IN ('minutes','hours','days')),
		concurrency          INTEGER NOT NULL DEFAULT 1,
		retry_attempts       INTEGER NOT NULL DEFAULT 3,
		retry_delay_seconds  INTEGER NOT NULL DEFAULT 2,
		enabled              INTEGER NOT NULL DEFAULT 1,
		include_filters      TEXT NOT NULL DEFAULT '[]',
		exclude_filters      TEXT NOT NULL DEFAULT '[]',
		include_path_filters TEXT NOT NULL DEFAULT '[]',
		include_name_filters TEXT NOT NULL DEFAULT '[]',
		exclude_path_filters TEXT NOT NULL DEFAULT '[]',
		exclude_name_filters TEXT NOT NULL DEFAULT '[]',
		run_retention_days   INTEGER NOT NULL DEFAULT 0,
		created_at           TEXT NOT NULL,
		updated_at           TEXT NOT NULL
	)`,

	`CREATE TABLE IF NOT EXISTS git_repos (
		id      TEXT PRIMARY KEY,
		job_id  TEXT NOT NULL REFERENCES sync_jobs(id) ON DELETE CASCADE,
		url     TEXT NOT NULL,
		branch  TEXT NOT NULL DEFAULT 'main'
	)`,

	`CREATE TABLE IF NOT EXISTS runs (
		id                   TEXT PRIMARY KEY,
		job_id               TEXT NOT NULL REFERENCES sync_jobs(id) ON DELETE CASCADE,
		status               TEXT NOT NULL CHECK(status IN ('running','completed','nothing_to_sync','failed','partial','canceled','server_stopped')),
		started_at           TEXT NOT NULL,
		finished_at          TEXT,
		total_files          INTEGER NOT NULL DEFAULT 0,
		copied_files         INTEGER NOT NULL DEFAULT 0,
		skipped_files        INTEGER NOT NULL DEFAULT 0,
		failed_files         INTEGER NOT NULL DEFAULT 0,
		total_size_bytes     INTEGER NOT NULL DEFAULT 0,
		bytes_copied         INTEGER NOT NULL DEFAULT 0,
		transfers_started_at TEXT,
		error_msg            TEXT
	)`,

	`CREATE TABLE IF NOT EXISTS transfers (
		id            TEXT PRIMARY KEY,
		run_id        TEXT NOT NULL REFERENCES runs(id) ON DELETE CASCADE,
		remote_path   TEXT NOT NULL,
		local_path    TEXT NOT NULL,
		size_bytes    INTEGER NOT NULL DEFAULT 0,
		bytes_xferred INTEGER NOT NULL DEFAULT 0,
		duration_ms   INTEGER,
		status        TEXT NOT NULL CHECK(status IN ('pending','in_progress','done','skipped','failed')),
		error_msg     TEXT,
		started_at    TEXT,
		finished_at   TEXT
	)`,

	// file_state is kept here so that pre-existing versioned migrations
	// (file_state_cascade_delete) can still run on fresh databases. The
	// rename_file_state_to_sync_state versioned migration moves its data
	// into sync_state and drops it.
	`CREATE TABLE IF NOT EXISTS file_state (
		job_id      TEXT NOT NULL REFERENCES sync_jobs(id) ON DELETE CASCADE,
		remote_path TEXT NOT NULL,
		size_bytes  INTEGER NOT NULL,
		mtime       TEXT NOT NULL,
		copied_at   TEXT NOT NULL,
		PRIMARY KEY (job_id, remote_path)
	)`,

	`CREATE TABLE IF NOT EXISTS sync_state (
		job_id       TEXT NOT NULL REFERENCES sync_jobs(id) ON DELETE CASCADE,
		remote_path  TEXT NOT NULL,
		size_bytes   INTEGER NOT NULL DEFAULT 0,
		mtime        TEXT,
		content_hash TEXT,
		copied_at    TEXT NOT NULL,
		PRIMARY KEY (job_id, remote_path)
	)`,

	`CREATE TABLE IF NOT EXISTS sessions (
		token        TEXT PRIMARY KEY,
		last_used_at TEXT NOT NULL
	)`,
}
