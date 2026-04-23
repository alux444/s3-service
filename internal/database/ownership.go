package database

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5/pgconn"
)

var ErrPolicyNotFound = errors.New("authorization policy not found")
var ErrBucketConnectionAlreadyExists = errors.New("bucket connection already exists")
var ErrBucketConnectionNotFound = errors.New("bucket connection not found")

type BucketConnection struct {
	BucketName      string   `json:"bucket_name"`
	Region          string   `json:"region"`
	RoleARN         string   `json:"role_arn"`
	ExternalID      *string  `json:"external_id"`
	AllowedPrefixes []string `json:"allowed_prefixes"`
}

type EffectiveAuthorizationPolicy struct {
	CanRead            bool
	CanWrite           bool
	CanDelete          bool
	CanList            bool
	ConnectionPrefixes []string
	PrincipalPrefixes  []string
}

type AuthorizationPolicyLookup struct {
	ProjectID     string
	AppID         string
	PrincipalType string
	PrincipalID   string
	BucketName    string
}

type OwnershipRepository struct {
	db *sql.DB
}

func NewOwnershipRepository(db *sql.DB) *OwnershipRepository {
	return &OwnershipRepository{db: db}
}

func (r *OwnershipRepository) ListActiveBucketsForConnectionScope(ctx context.Context, projectID string, appID string) ([]BucketConnection, error) {
	return ListActiveBucketsForConnectionScope(ctx, r.db, projectID, appID)
}

func (r *OwnershipRepository) CreateBucketConnection(
	ctx context.Context,
	projectID string,
	appID string,
	bucketName string,
	region string,
	roleARN string,
	externalID *string,
	allowedPrefixes []string,
) error {
	return CreateBucketConnection(ctx, r.db, projectID, appID, bucketName, region, roleARN, externalID, allowedPrefixes)
}

func (r *OwnershipRepository) GetEffectiveAuthorizationPolicy(ctx context.Context, lookup AuthorizationPolicyLookup) (EffectiveAuthorizationPolicy, error) {
	return GetEffectiveAuthorizationPolicy(ctx, r.db, lookup)
}

func (r *OwnershipRepository) UpsertAccessPolicyForConnectionScope(
	ctx context.Context,
	projectID string,
	appID string,
	bucketName string,
	principalType string,
	principalID string,
	role string,
	canRead bool,
	canWrite bool,
	canDelete bool,
	canList bool,
	prefixAllowlist []string,
) error {
	return UpsertAccessPolicyForConnectionScope(ctx, r.db, projectID, appID, bucketName, principalType, principalID, role, canRead, canWrite, canDelete, canList, prefixAllowlist)
}

func ListActiveBucketsForConnectionScope(ctx context.Context, db *sql.DB, projectID string, appID string) ([]BucketConnection, error) {
	if projectID == "" || appID == "" {
		return nil, fmt.Errorf("projectID and appID must be provided")
	}

	rows, err := db.QueryContext(ctx, `
		SELECT bucket_name, region, role_arn, external_id, allowed_prefixes
		FROM bucket_connections 
		WHERE project_id = $1 AND app_id = $2 AND is_active = true
		ORDER BY created_at DESC
	`, projectID, appID)
	if err != nil {
		return nil, fmt.Errorf("failed to query active bucket connections: %w", err)
	}
	defer rows.Close()

	var buckets []BucketConnection
	for rows.Next() {
		var bucket BucketConnection
		var allowedPrefixesRaw any
		if err := rows.Scan(&bucket.BucketName, &bucket.Region, &bucket.RoleARN, &bucket.ExternalID, &allowedPrefixesRaw); err != nil {
			return nil, fmt.Errorf("failed to scan bucket connection row: %w", err)
		}
		allowedPrefixes, err := decodeStringSliceDBValue(allowedPrefixesRaw)
		if err != nil {
			return nil, fmt.Errorf("failed to decode allowed_prefixes: %w", err)
		}
		bucket.AllowedPrefixes = allowedPrefixes
		buckets = append(buckets, bucket)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating bucket connection rows: %w", err)
	}

	return buckets, nil
}

func CreateBucketConnection(
	ctx context.Context,
	db *sql.DB,
	projectID string,
	appID string,
	bucketName string,
	region string,
	roleARN string,
	externalID *string,
	allowedPrefixes []string,
) error {
	if projectID == "" || appID == "" || bucketName == "" || region == "" || roleARN == "" {
		return fmt.Errorf("projectID, appID, bucketName, region, and roleARN must be provided")
	}

	_, err := db.ExecContext(ctx, `
		INSERT INTO bucket_connections (
			project_id,
			app_id,
			bucket_name,
			region,
			role_arn,
			external_id,
			allowed_prefixes,
			is_active
		) VALUES ($1, $2, $3, $4, $5, $6, $7, true)
	`, projectID, appID, bucketName, region, roleARN, externalID, allowedPrefixes)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			return ErrBucketConnectionAlreadyExists
		}
		return fmt.Errorf("failed to create bucket connection: %w", err)
	}

	return nil
}

func GetEffectiveAuthorizationPolicy(ctx context.Context, db *sql.DB, lookup AuthorizationPolicyLookup) (EffectiveAuthorizationPolicy, error) {
	if lookup.ProjectID == "" || lookup.AppID == "" || lookup.PrincipalType == "" || lookup.PrincipalID == "" || lookup.BucketName == "" {
		return EffectiveAuthorizationPolicy{}, fmt.Errorf("projectID, appID, principalType, principalID, and bucketName must be provided")
	}

	query := `
		SELECT ap.can_read, ap.can_write, ap.can_delete, ap.can_list, bc.allowed_prefixes, ap.prefix_allowlist 
		FROM bucket_connections bc 
		JOIN access_policies ap ON ap.bucket_connection_id = bc.id 
		WHERE bc.project_id = $1 
		AND bc.app_id = $2 
		AND bc.bucket_name = $3 
		AND bc.is_active = true
		AND ap.principal_type = $4
		AND ap.principal_id = $5 
		LIMIT 1
		`

	var policy EffectiveAuthorizationPolicy
	var connectionPrefixesRaw any
	var principalPrefixesRaw any
	err := db.QueryRowContext(ctx, query, lookup.ProjectID, lookup.AppID, lookup.BucketName, lookup.PrincipalType, lookup.PrincipalID).Scan(
		&policy.CanRead,
		&policy.CanWrite,
		&policy.CanDelete,
		&policy.CanList,
		&connectionPrefixesRaw,
		&principalPrefixesRaw,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return EffectiveAuthorizationPolicy{}, ErrPolicyNotFound
		}
		return EffectiveAuthorizationPolicy{}, fmt.Errorf("failed to query effective authorization policy: %w", err)
	}
	connectionPrefixes, err := decodeStringSliceDBValue(connectionPrefixesRaw)
	if err != nil {
		return EffectiveAuthorizationPolicy{}, fmt.Errorf("failed to decode connection prefixes: %w", err)
	}
	principalPrefixes, err := decodeStringSliceDBValue(principalPrefixesRaw)
	if err != nil {
		return EffectiveAuthorizationPolicy{}, fmt.Errorf("failed to decode principal prefixes: %w", err)
	}
	policy.ConnectionPrefixes = connectionPrefixes
	policy.PrincipalPrefixes = principalPrefixes

	return policy, nil
}

func decodeStringSliceDBValue(raw any) ([]string, error) {
	switch v := raw.(type) {
	case nil:
		return []string{}, nil
	case []string:
		return v, nil
	case string:
		return parseStringSliceValue(v)
	case []byte:
		return parseStringSliceValue(string(v))
	default:
		return nil, fmt.Errorf("unsupported value type %T", raw)
	}
}

func parseStringSliceValue(value string) ([]string, error) {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" || trimmed == "{}" || trimmed == "[]" {
		return []string{}, nil
	}
	// handle JSON arrays
	if strings.HasPrefix(trimmed, "[") {
		var values []string
		if err := json.Unmarshal([]byte(trimmed), &values); err != nil {
			return nil, fmt.Errorf("invalid JSON array format: %w", err)
		}
		return values, nil
	}
	if strings.HasPrefix(trimmed, "{") && strings.HasSuffix(trimmed, "}") {
		return parsePostgresArrayLiteral(trimmed), nil
	}

	parts := strings.Split(trimmed, ",")
	values := make([]string, 0, len(parts))
	for _, part := range parts {
		p := strings.TrimSpace(part)
		if p != "" {
			values = append(values, p)
		}
	}
	return values, nil
}

// examples: {prefix1, prefix2}, {"prefix1","prefix2"}
func parsePostgresArrayLiteral(literal string) []string {
	body := strings.TrimSuffix(strings.TrimPrefix(literal, "{"), "}")
	if strings.TrimSpace(body) == "" {
		return []string{}
	}

	values := make([]string, 0)
	var current strings.Builder
	inQuotes := false
	escaped := false
	for _, r := range body {
		switch {
		case escaped:
			current.WriteRune(r)
			escaped = false
		case r == '\\':
			escaped = true
		case r == '"':
			inQuotes = !inQuotes
		case r == ',' && !inQuotes:
			item := strings.TrimSpace(current.String())
			if item != "" {
				values = append(values, item)
			}
			current.Reset()
		default:
			current.WriteRune(r)
		}
	}

	item := strings.TrimSpace(current.String())
	if item != "" {
		values = append(values, item)
	}
	return values
}

func UpsertAccessPolicyForConnectionScope(
	ctx context.Context,
	db *sql.DB,
	projectID string,
	appID string,
	bucketName string,
	principalType string,
	principalID string,
	role string,
	canRead bool,
	canWrite bool,
	canDelete bool,
	canList bool,
	prefixAllowlist []string,
) error {
	if projectID == "" || appID == "" || bucketName == "" || principalType == "" || principalID == "" || role == "" {
		return fmt.Errorf("projectID, appID, bucketName, principalType, principalID, and role must be provided")
	}

	result, err := db.ExecContext(ctx, `
		INSERT INTO access_policies (
			bucket_connection_id,
			principal_type,
			principal_id,
			role,
			can_read,
			can_write,
			can_delete,
			can_list,
			prefix_allowlist
		)
		SELECT
			bc.id,
			$4,
			$5,
			$6,
			$7,
			$8,
			$9,
			$10,
			$11
		FROM bucket_connections bc
		WHERE bc.project_id = $1
		  AND bc.app_id = $2
		  AND bc.bucket_name = $3
		  AND bc.is_active = true
		ON CONFLICT (bucket_connection_id, principal_type, principal_id)
		DO UPDATE SET
			role = EXCLUDED.role,
			can_read = EXCLUDED.can_read,
			can_write = EXCLUDED.can_write,
			can_delete = EXCLUDED.can_delete,
			can_list = EXCLUDED.can_list,
			prefix_allowlist = EXCLUDED.prefix_allowlist,
			updated_at = NOW()
	`, projectID, appID, bucketName, principalType, principalID, role, canRead, canWrite, canDelete, canList, prefixAllowlist)
	if err != nil {
		return fmt.Errorf("failed to upsert access policy: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to check upsert access policy rows affected: %w", err)
	}

	if rowsAffected == 0 {
		return ErrBucketConnectionNotFound
	}

	return nil
}
