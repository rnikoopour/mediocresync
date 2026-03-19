package db

import (
	"database/sql"
	"fmt"
)

func migrate(db *sql.DB) error {
	for i, stmt := range schema {
		if _, err := db.Exec(stmt); err != nil {
			return fmt.Errorf("migration statement %d: %w", i, err)
		}
	}
	return nil
}
