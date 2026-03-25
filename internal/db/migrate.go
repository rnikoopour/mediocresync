package db

import (
	"database/sql"
	"fmt"
	"strings"
)

// versionedMigrations are run exactly once, tracked by key in the _meta table.
// Use this for changes that can't be expressed as idempotent ALTER TABLE statements
// (e.g. rebuilding a table to change a CHECK constraint).
var versionedMigrations = []struct {
	key   string
	stmts []string
}{
	{
		key: "runs_allow_canceled",
		stmts: []string{
			`PRAGMA foreign_keys=OFF`,
			`DROP TABLE IF EXISTS runs_new`,
			`CREATE TABLE runs_new (
				id               TEXT PRIMARY KEY,
				job_id           TEXT NOT NULL REFERENCES sync_jobs(id),
				status           TEXT NOT NULL CHECK(status IN ('running','completed','failed','canceled')),
				started_at       TEXT NOT NULL,
				finished_at      TEXT,
				total_files      INTEGER NOT NULL DEFAULT 0,
				copied_files     INTEGER NOT NULL DEFAULT 0,
				skipped_files    INTEGER NOT NULL DEFAULT 0,
				failed_files     INTEGER NOT NULL DEFAULT 0,
				total_size_bytes INTEGER NOT NULL DEFAULT 0,
				error_msg        TEXT
			)`,
			`INSERT INTO runs_new SELECT id,job_id,status,started_at,finished_at,total_files,copied_files,skipped_files,failed_files,total_size_bytes,NULL FROM runs`,
			`DROP TABLE runs`,
			`ALTER TABLE runs_new RENAME TO runs`,
			`PRAGMA foreign_keys=ON`,
		},
	},
	{
		key: "runs_allow_server_stopped",
		stmts: []string{
			`PRAGMA foreign_keys=OFF`,
			`DROP TABLE IF EXISTS runs_new`,
			`CREATE TABLE runs_new (
				id               TEXT PRIMARY KEY,
				job_id           TEXT NOT NULL REFERENCES sync_jobs(id),
				status           TEXT NOT NULL CHECK(status IN ('running','completed','failed','canceled','server_stopped')),
				started_at       TEXT NOT NULL,
				finished_at      TEXT,
				total_files      INTEGER NOT NULL DEFAULT 0,
				copied_files     INTEGER NOT NULL DEFAULT 0,
				skipped_files    INTEGER NOT NULL DEFAULT 0,
				failed_files     INTEGER NOT NULL DEFAULT 0,
				total_size_bytes INTEGER NOT NULL DEFAULT 0,
				error_msg        TEXT
			)`,
			`INSERT INTO runs_new SELECT id,job_id,status,started_at,finished_at,total_files,copied_files,skipped_files,failed_files,total_size_bytes,NULL FROM runs`,
			`DROP TABLE runs`,
			`ALTER TABLE runs_new RENAME TO runs`,
			`PRAGMA foreign_keys=ON`,
		},
	},
	{
		key: "runs_allow_nothing_to_sync",
		stmts: []string{
			`PRAGMA foreign_keys=OFF`,
			`DROP TABLE IF EXISTS runs_new`,
			`CREATE TABLE runs_new (
				id               TEXT PRIMARY KEY,
				job_id           TEXT NOT NULL REFERENCES sync_jobs(id),
				status           TEXT NOT NULL CHECK(status IN ('running','completed','nothing_to_sync','failed','canceled','server_stopped')),
				started_at       TEXT NOT NULL,
				finished_at      TEXT,
				total_files      INTEGER NOT NULL DEFAULT 0,
				copied_files     INTEGER NOT NULL DEFAULT 0,
				skipped_files    INTEGER NOT NULL DEFAULT 0,
				failed_files     INTEGER NOT NULL DEFAULT 0,
				total_size_bytes INTEGER NOT NULL DEFAULT 0,
				error_msg        TEXT
			)`,
			`INSERT INTO runs_new SELECT id,job_id,status,started_at,finished_at,total_files,copied_files,skipped_files,failed_files,total_size_bytes,NULL FROM runs`,
			`DROP TABLE runs`,
			`ALTER TABLE runs_new RENAME TO runs`,
			`PRAGMA foreign_keys=ON`,
		},
	},
	{
		key: "transfers_cascade_delete",
		stmts: []string{
			`PRAGMA foreign_keys=OFF`,
			`DROP TABLE IF EXISTS transfers_new`,
			`CREATE TABLE transfers_new (
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
			`INSERT INTO transfers_new SELECT * FROM transfers`,
			`DROP TABLE transfers`,
			`ALTER TABLE transfers_new RENAME TO transfers`,
			`PRAGMA foreign_keys=ON`,
		},
	},
	{
		key: "file_state_cascade_delete",
		stmts: []string{
			`PRAGMA foreign_keys=OFF`,
			`DROP TABLE IF EXISTS file_state_new`,
			`CREATE TABLE file_state_new (
				job_id      TEXT NOT NULL REFERENCES sync_jobs(id) ON DELETE CASCADE,
				remote_path TEXT NOT NULL,
				size_bytes  INTEGER NOT NULL,
				mtime       TEXT NOT NULL,
				copied_at   TEXT NOT NULL,
				PRIMARY KEY (job_id, remote_path)
			)`,
			`INSERT INTO file_state_new SELECT * FROM file_state`,
			`DROP TABLE file_state`,
			`ALTER TABLE file_state_new RENAME TO file_state`,
			`PRAGMA foreign_keys=ON`,
		},
	},
	{
		key: "runs_cascade_delete",
		stmts: []string{
			`PRAGMA foreign_keys=OFF`,
			`DROP TABLE IF EXISTS runs_new`,
			`CREATE TABLE runs_new (
				id               TEXT PRIMARY KEY,
				job_id           TEXT NOT NULL REFERENCES sync_jobs(id) ON DELETE CASCADE,
				status           TEXT NOT NULL CHECK(status IN ('running','completed','nothing_to_sync','failed','canceled','server_stopped')),
				started_at       TEXT NOT NULL,
				finished_at      TEXT,
				total_files      INTEGER NOT NULL DEFAULT 0,
				copied_files     INTEGER NOT NULL DEFAULT 0,
				skipped_files    INTEGER NOT NULL DEFAULT 0,
				failed_files     INTEGER NOT NULL DEFAULT 0,
				total_size_bytes INTEGER NOT NULL DEFAULT 0,
				error_msg        TEXT
			)`,
			`INSERT INTO runs_new SELECT id,job_id,status,started_at,finished_at,total_files,copied_files,skipped_files,failed_files,total_size_bytes,error_msg FROM runs`,
			`DROP TABLE runs`,
			`ALTER TABLE runs_new RENAME TO runs`,
			`PRAGMA foreign_keys=ON`,
		},
	},
	{
		key: "runs_allow_partial",
		stmts: []string{
			`PRAGMA foreign_keys=OFF`,
			`DROP TABLE IF EXISTS runs_new`,
			`CREATE TABLE runs_new (
				id               TEXT PRIMARY KEY,
				job_id           TEXT NOT NULL REFERENCES sync_jobs(id) ON DELETE CASCADE,
				status           TEXT NOT NULL CHECK(status IN ('running','completed','nothing_to_sync','failed','partial','canceled','server_stopped')),
				started_at       TEXT NOT NULL,
				finished_at      TEXT,
				total_files      INTEGER NOT NULL DEFAULT 0,
				copied_files     INTEGER NOT NULL DEFAULT 0,
				skipped_files    INTEGER NOT NULL DEFAULT 0,
				failed_files     INTEGER NOT NULL DEFAULT 0,
				total_size_bytes INTEGER NOT NULL DEFAULT 0,
				error_msg        TEXT
			)`,
			`INSERT INTO runs_new SELECT id,job_id,status,started_at,finished_at,total_files,copied_files,skipped_files,failed_files,total_size_bytes,error_msg FROM runs`,
			`DROP TABLE runs`,
			`ALTER TABLE runs_new RENAME TO runs`,
			`PRAGMA foreign_keys=ON`,
		},
	},
	{
		// Create the sources table and migrate all existing FTPES connections into it.
		key: "create_sources_table",
		stmts: []string{
			`PRAGMA foreign_keys=OFF`,
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
			`INSERT OR IGNORE INTO sources (id, name, type, host, port, username, password, skip_tls_verify, enable_epsv, created_at, updated_at)
			 SELECT id, name, 'ftpes', host, port, username, password, skip_tls_verify, enable_epsv, created_at, updated_at
			 FROM connections`,
			`PRAGMA foreign_keys=ON`,
		},
	},
	{
		key: "create_git_repos_table",
		stmts: []string{
			`CREATE TABLE IF NOT EXISTS git_repos (
				id      TEXT PRIMARY KEY,
				job_id  TEXT NOT NULL REFERENCES sync_jobs(id) ON DELETE CASCADE,
				url     TEXT NOT NULL,
				branch  TEXT NOT NULL DEFAULT 'main'
			)`,
		},
	},
	{
		// Rebuild sync_jobs replacing connection_id with source_id.
		key: "sync_jobs_use_source_id",
		stmts: []string{
			`PRAGMA foreign_keys=OFF`,
			`DROP TABLE IF EXISTS sync_jobs_new`,
			`CREATE TABLE sync_jobs_new (
				id                   TEXT PRIMARY KEY,
				name                 TEXT NOT NULL,
				source_id            TEXT NOT NULL REFERENCES sources(id),
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
			`INSERT INTO sync_jobs_new (id, name, source_id, remote_path, local_dest, interval_value, interval_unit, concurrency, retry_attempts, retry_delay_seconds, enabled, include_filters, exclude_filters, include_path_filters, include_name_filters, exclude_path_filters, exclude_name_filters, run_retention_days, created_at, updated_at)
			 SELECT id, name, connection_id, remote_path, local_dest, interval_value, interval_unit, concurrency, retry_attempts, retry_delay_seconds, enabled, include_filters, exclude_filters, include_path_filters, include_name_filters, exclude_path_filters, exclude_name_filters, run_retention_days, created_at, updated_at
			 FROM sync_jobs`,
			`DROP TABLE sync_jobs`,
			`ALTER TABLE sync_jobs_new RENAME TO sync_jobs`,
			`PRAGMA foreign_keys=ON`,
		},
	},
	{
		key: "drop_connections_table",
		stmts: []string{
			`PRAGMA foreign_keys=OFF`,
			`DROP TABLE IF EXISTS connections`,
			`PRAGMA foreign_keys=ON`,
		},
	},
	{
		// Rename file_state to sync_state and add content_hash for Git fingerprinting.
		// mtime is made nullable since Git repos do not have a meaningful mtime.
		key: "rename_file_state_to_sync_state",
		stmts: []string{
			`PRAGMA foreign_keys=OFF`,
			`DROP TABLE IF EXISTS sync_state`,
			`CREATE TABLE sync_state (
				job_id       TEXT NOT NULL REFERENCES sync_jobs(id) ON DELETE CASCADE,
				remote_path  TEXT NOT NULL,
				size_bytes   INTEGER NOT NULL DEFAULT 0,
				mtime        TEXT,
				content_hash TEXT,
				copied_at    TEXT NOT NULL,
				PRIMARY KEY (job_id, remote_path)
			)`,
			`INSERT INTO sync_state (job_id, remote_path, size_bytes, mtime, copied_at)
			 SELECT job_id, remote_path, size_bytes, mtime, copied_at FROM file_state`,
			`DROP TABLE file_state`,
			`PRAGMA foreign_keys=ON`,
		},
	},
}

func migrate(db *sql.DB) error {
	for i, stmt := range schema {
		if _, err := db.Exec(stmt); err != nil {
			return fmt.Errorf("migration statement %d: %w", i, err)
		}
	}
	for _, stmt := range alterations {
		if _, err := db.Exec(stmt); err != nil {
			// Ignore "duplicate column name" (column already exists) and
			// "no such table" (table was dropped by a versioned migration).
			if !strings.Contains(err.Error(), "duplicate column name") &&
				!strings.Contains(err.Error(), "no such table") {
				return fmt.Errorf("alteration %q: %w", stmt, err)
			}
		}
	}
	for _, m := range versionedMigrations {
		var count int
		if err := db.QueryRow(`SELECT COUNT(*) FROM _meta WHERE key=?`, "migration:"+m.key).Scan(&count); err != nil {
			return fmt.Errorf("check migration %q: %w", m.key, err)
		}
		if count > 0 {
			continue
		}
		for _, stmt := range m.stmts {
			if _, err := db.Exec(stmt); err != nil {
				return fmt.Errorf("versioned migration %q: %w", m.key, err)
			}
		}
		if _, err := db.Exec(`INSERT INTO _meta (key, value) VALUES (?, '1')`, "migration:"+m.key); err != nil {
			return fmt.Errorf("record migration %q: %w", m.key, err)
		}
	}
	return nil
}
