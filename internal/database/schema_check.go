package database

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/pressly/goose/v3"
)

var requiredTables = []string{
	"public.bucket_connections",
	"public.access_policies",
	"public.audit_events",
}

func CheckSchema(ctx context.Context, db *sql.DB) error {
	for _, table := range requiredTables {
		var exists bool
		if err := db.QueryRowContext(ctx, "SELECT to_regclass($1) IS NOT NULL", table).Scan(&exists); err != nil {
			return fmt.Errorf("failed to check for table %s: %w", table, err)
		}
		if !exists {
			return fmt.Errorf("required table %s does not exist", table)
		}
	}

	version, err := goose.GetDBVersion(db)
	if err != nil {
		return fmt.Errorf("failed to get database schema version: %w", err)
	}

	if version < 1 {
		return fmt.Errorf("database schema version is %d, but at least version 1 is required", version)
	}

	return nil
}
