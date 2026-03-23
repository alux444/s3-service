package database

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/pressly/goose/v3"
)

func MigrateUp(ctx context.Context, db *sql.DB, migrationsDir string) error {
	if err := goose.SetDialect("postgres"); err != nil {
		return fmt.Errorf("failed to set goose dialect: %w", err)
	}

	if err := goose.Up(db, migrationsDir); err != nil {
		return fmt.Errorf("failed to apply database migrations: %w", err)
	}

	return nil
}
