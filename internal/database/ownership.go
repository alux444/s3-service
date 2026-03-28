package database

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
)

var ErrPolicyNotFound = errors.New("authorization policy not found")

type EffectiveAuthorizationPolicy struct {
	CanRead            bool
	CanWrite           bool
	CanDelete          bool
	CanList            bool
	ConnectionPrefixes []string
	PrincipalPrefixes  []string
}

type OwnershipRepository struct {
	db *sql.DB
}

func NewOwnershipRepository(db *sql.DB) *OwnershipRepository {
	return &OwnershipRepository{db: db}
}

func (r *OwnershipRepository) ListActiveBucketsForConnectionScope(ctx context.Context, projectID string, appID string) ([]string, error) {
	return ListActiveBucketsForConnectionScope(ctx, r.db, projectID, appID)
}

func (r *OwnershipRepository) GetEffectiveAuthorizationPolicy(ctx context.Context, projectID string, appID string, principalID string, bucketName string) (EffectiveAuthorizationPolicy, error) {
	return GetEffectiveAuthorizationPolicy(ctx, r.db, projectID, appID, principalID, bucketName)
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

func GetEffectiveAuthorizationPolicy(ctx context.Context, db *sql.DB, projectID string, appID string, principalID string, bucketName string) (EffectiveAuthorizationPolicy, error) {
	if projectID == "" || appID == "" || principalID == "" || bucketName == "" {
		return EffectiveAuthorizationPolicy{}, fmt.Errorf("projectID, appID, principalID, and bucketName must be provided")
	}

	query := `
		SELECT ap.can_read, ap.can_write, ap.can_delete, ap.can_list, bc.allowed_prefixes, ap.prefix_allowlist 
		FROM bucket_connections bc 
		JOIN access_policies ap ON ap.bucket_connection_id = bc.id 
		WHERE bc.project_id = $1 
		AND bc.app_id = $2 
		AND bc.bucket_name = $3 
		AND bc.is_active = true
		AND ap.principal_id = $4 
		LIMIT 1
		`

	var policy EffectiveAuthorizationPolicy
	err := db.QueryRowContext(ctx, query, projectID, appID, bucketName, principalID).Scan(
		&policy.CanRead,
		&policy.CanWrite,
		&policy.CanDelete,
		&policy.CanList,
		&policy.ConnectionPrefixes,
		&policy.PrincipalPrefixes,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return EffectiveAuthorizationPolicy{}, ErrPolicyNotFound
		}
		return EffectiveAuthorizationPolicy{}, fmt.Errorf("failed to query effective authorization policy: %w", err)
	}

	return policy, nil
}
