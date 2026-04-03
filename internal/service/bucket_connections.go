package service

import (
	"context"
	"errors"
	"fmt"
)

var ErrInvalidBucketConnectionInput = errors.New("invalid bucket connection input")

type BucketConnectionsRepository interface {
	ListActiveBucketsForConnectionScope(ctx context.Context, projectID string, appID string) ([]string, error)
	CreateBucketConnection(ctx context.Context, projectID string, appID string, bucketName string, region string, roleARN string, externalID *string, allowedPrefixes []string) error
}

type BucketConnectionsService struct {
	repo BucketConnectionsRepository
}

func NewBucketConnectionsService(repo BucketConnectionsRepository) *BucketConnectionsService {
	return &BucketConnectionsService{repo: repo}
}

func (s *BucketConnectionsService) ListForScope(ctx context.Context, projectID, appID string) ([]string, error) {
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

	return s.repo.CreateBucketConnection(ctx, projectID, appID, bucketName, region, roleARN, externalID, allowedPrefixes)
}
