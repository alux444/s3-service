package service

import (
	"context"
	"errors"
	"fmt"

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
	return s.repo.ListActiveBucketsForConnectionScope(ctx, projectID, appID)
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
	if projectID == "" || appID == "" || bucketName == "" || region == "" || roleARN == "" {
		return fmt.Errorf("%w: projectID, appID, bucketName, region, and roleARN are required", ErrInvalidBucketConnectionInput)
	}

	if s.validator != nil {
		if err := s.validator.ValidateBucketConnection(ctx, bucketName, region, roleARN, externalID); err != nil {
			return err
		}
	}

	return s.repo.CreateBucketConnection(ctx, projectID, appID, bucketName, region, roleARN, externalID, allowedPrefixes)
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
	if projectID == "" || appID == "" || bucketName == "" || principalType == "" || principalID == "" || role == "" {
		return fmt.Errorf("%w: projectID, appID, bucketName, principalType, principalID, and role are required", ErrInvalidAccessPolicyInput)
	}

	if _, err := auth.ParsePrincipalType(principalType); err != nil {
		return fmt.Errorf("%w: invalid principal type", ErrInvalidAccessPolicyInput)
	}

	if _, err := auth.ParseRole(role); err != nil {
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
		if errors.Is(err, database.ErrBucketConnectionNotFound) {
			return fmt.Errorf("%w: %s", ErrBucketConnectionNotFound, bucketName)
		}
		return err
	}

	return nil
}
