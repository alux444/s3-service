package service

import (
	"context"
	"errors"
	"fmt"
	"log/slog"

	"s3-service/internal/auth"
	"s3-service/internal/database"
)

var ErrInvalidBucketConnectionInput = errors.New("invalid bucket connection input")
var ErrInvalidAccessPolicyInput = errors.New("invalid access policy input")

type BucketConnectionsRepository interface {
	ListActiveBucketsForConnectionScope(ctx context.Context, projectID string, appID string) ([]database.BucketConnection, error)
	CreateBucketConnection(ctx context.Context, projectID string, appID string, bucketName string, region string, roleARN string, externalID *string, allowedPrefixes []string) error
	UpsertAccessPolicyForConnectionScope(ctx context.Context, projectID string, appID string, bucketName string, principalType string, principalID string, role string, canRead bool, canWrite bool, canDelete bool, canList bool, prefixAllowlist []string) error
}

type BucketConnectionSecurityValidator interface {
	ValidateBucketConnection(ctx context.Context, bucketName string, region string, roleARN string, externalID *string) error
}

type BucketConnectionsServiceOption func(*BucketConnectionsService)

func WithBucketConnectionSecurityValidator(validator BucketConnectionSecurityValidator) BucketConnectionsServiceOption {
	return func(s *BucketConnectionsService) {
		s.validator = validator
	}
}

type BucketConnectionsService struct {
	repo      BucketConnectionsRepository
	validator BucketConnectionSecurityValidator
}

func NewBucketConnectionsService(repo BucketConnectionsRepository, opts ...BucketConnectionsServiceOption) *BucketConnectionsService {
	svc := &BucketConnectionsService{repo: repo}
	for _, opt := range opts {
		opt(svc)
	}
	return svc
}

func (s *BucketConnectionsService) ListForScope(ctx context.Context, projectID, appID string) ([]database.BucketConnection, error) {
	slog.Info("bucket_connections_list_started",
		"project_id", projectID,
		"app_id", appID,
	)
	buckets, err := s.repo.ListActiveBucketsForConnectionScope(ctx, projectID, appID)
	if err != nil {
		slog.Info("bucket_connections_list_failed",
			"project_id", projectID,
			"app_id", appID,
			"error", err,
		)
		return nil, err
	}
	slog.Info("bucket_connections_list_completed",
		"project_id", projectID,
		"app_id", appID,
		"bucket_count", len(buckets),
	)
	return buckets, nil
}

func (s *BucketConnectionsService) CreateForScope(
	ctx context.Context,
	projectID string,
	appID string,
	bucketName string,
	region string,
	roleARN string,
	externalID *string,
	allowedPrefixes []string,
) error {
	slog.Info("bucket_connections_create_started",
		"project_id", projectID,
		"app_id", appID,
		"bucket_name", bucketName,
		"region", region,
	)
	if projectID == "" || appID == "" || bucketName == "" || region == "" || roleARN == "" {
		slog.Info("bucket_connections_create_invalid_input",
			"project_id", projectID,
			"app_id", appID,
			"bucket_name", bucketName,
			"region", region,
		)
		return fmt.Errorf("%w: projectID, appID, bucketName, region, and roleARN are required", ErrInvalidBucketConnectionInput)
	}

	if s.validator != nil {
		if err := s.validator.ValidateBucketConnection(ctx, bucketName, region, roleARN, externalID); err != nil {
			slog.Info("bucket_connections_create_validation_failed",
				"project_id", projectID,
				"app_id", appID,
				"bucket_name", bucketName,
				"region", region,
				"error", err,
			)
			return err
		}
	}

	if err := s.repo.CreateBucketConnection(ctx, projectID, appID, bucketName, region, roleARN, externalID, allowedPrefixes); err != nil {
		slog.Info("bucket_connections_create_failed",
			"project_id", projectID,
			"app_id", appID,
			"bucket_name", bucketName,
			"region", region,
			"error", err,
		)
		return err
	}
	slog.Info("bucket_connections_create_completed",
		"project_id", projectID,
		"app_id", appID,
		"bucket_name", bucketName,
		"region", region,
	)
	return nil
}

func (s *BucketConnectionsService) UpsertAccessPolicyForScope(
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
	slog.Info("access_policy_upsert_started",
		"project_id", projectID,
		"app_id", appID,
		"bucket_name", bucketName,
		"principal_type", principalType,
		"principal_id", principalID,
		"role", role,
	)
	if projectID == "" || appID == "" || bucketName == "" || principalType == "" || principalID == "" || role == "" {
		slog.Info("access_policy_upsert_invalid_input",
			"project_id", projectID,
			"app_id", appID,
			"bucket_name", bucketName,
			"principal_type", principalType,
			"principal_id", principalID,
			"role", role,
		)
		return fmt.Errorf("%w: projectID, appID, bucketName, principalType, principalID, and role are required", ErrInvalidAccessPolicyInput)
	}

	if _, err := auth.ParsePrincipalType(principalType); err != nil {
		slog.Info("access_policy_upsert_invalid_principal_type",
			"principal_type", principalType,
		)
		return fmt.Errorf("%w: invalid principal type", ErrInvalidAccessPolicyInput)
	}

	if _, err := auth.ParseRole(role); err != nil {
		slog.Info("access_policy_upsert_invalid_role",
			"role", role,
		)
		return fmt.Errorf("%w: invalid role", ErrInvalidAccessPolicyInput)
	}

	err := s.repo.UpsertAccessPolicyForConnectionScope(
		ctx,
		projectID,
		appID,
		bucketName,
		principalType,
		principalID,
		role,
		canRead,
		canWrite,
		canDelete,
		canList,
		prefixAllowlist,
	)
	if err != nil {
		slog.Info("access_policy_upsert_failed",
			"project_id", projectID,
			"app_id", appID,
			"bucket_name", bucketName,
			"principal_type", principalType,
			"principal_id", principalID,
			"role", role,
			"error", err,
		)
		if errors.Is(err, database.ErrBucketConnectionNotFound) {
			return fmt.Errorf("%w: %s", ErrBucketConnectionNotFound, bucketName)
		}
		return err
	}

	slog.Info("access_policy_upsert_completed",
		"project_id", projectID,
		"app_id", appID,
		"bucket_name", bucketName,
		"principal_type", principalType,
		"principal_id", principalID,
		"role", role,
	)
	return nil
}
