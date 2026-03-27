package database

import (
	"context"
	"database/sql"
	"fmt"
)

type OwnershipRepository struct {
	db *sql.DB
}

func NewOwnershipRepository(db *sql.DB) *OwnershipRepository {
	return &OwnershipRepository{db: db}
}

func (r *OwnershipRepository) ListActiveBucketsForConnectionScope(ctx context.Context, projectID string, appID string) ([]string, error) {
	return ListActiveBucketsForConnectionScope(ctx, r.db, projectID, appID)
}

func ListActiveBucketsForConnectionScope(ctx context.Context, db *sql.DB, projectID string, appID string) ([]string, error) {
	if projectID == "" || appID == "" {
		return nil, fmt.Errorf("projectID and appID must be provided")
	}

	rows, err := db.QueryContext(ctx, `
		SELECT bucket_name 
		FROM bucket_connections 
		WHERE project_id = $1 AND app_id = $2 AND is_active = true
		ORDER BY created_at DESC
	`, projectID, appID)
	if err != nil {
		return nil, fmt.Errorf("failed to query active bucket connections: %w", err)
	}
	defer rows.Close()

	var buckets []string
	for rows.Next() {
		var bucketName string
		if err := rows.Scan(&bucketName); err != nil {
			return nil, fmt.Errorf("failed to scan bucket connection row: %w", err)
		}
		buckets = append(buckets, bucketName)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating bucket connection rows: %w", err)
	}

	return buckets, nil
}
