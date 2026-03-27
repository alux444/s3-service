package service

import "context"

type BucketConnectionsRepository interface {
	ListActiveBucketsForConnectionScope(ctx context.Context, projectID string, appID string) ([]string, error)
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
