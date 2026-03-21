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
				total_size_bytes INTEGER NOT NULL DEFAULT 0
			)`,
			`INSERT INTO runs_new SELECT * FROM runs`,
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
				total_size_bytes INTEGER NOT NULL DEFAULT 0
			)`,
			`INSERT INTO runs_new SELECT * FROM runs`,
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
				total_size_bytes INTEGER NOT NULL DEFAULT 0
			)`,
			`INSERT INTO runs_new SELECT * FROM runs`,
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
				total_size_bytes INTEGER NOT NULL DEFAULT 0
			)`,
			`INSERT INTO runs_new SELECT * FROM runs`,
			`DROP TABLE runs`,
			`ALTER TABLE runs_new RENAME TO runs`,
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
			// SQLite returns "duplicate column name" when the column already exists.
			if !strings.Contains(err.Error(), "duplicate column name") {
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
