package db

import (
	"database/sql"
	"fmt"
	"strings"
)

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
	return nil
}
