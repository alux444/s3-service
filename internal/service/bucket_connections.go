package service

import (
	"context"
	"errors"
	"fmt"

	"s3-service/internal/database"
)

var ErrInvalidBucketConnectionInput = errors.New("invalid bucket connection input")

type BucketConnectionsRepository interface {
	ListActiveBucketsForConnectionScope(ctx context.Context, projectID string, appID string) ([]database.BucketConnection, error)
	CreateBucketConnection(ctx context.Context, projectID string, appID string, bucketName string, region string, roleARN string, externalID *string, allowedPrefixes []string) error
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
