package service

import (
	"context"
	"errors"
	"testing"
)

type stubBucketConnectionsRepo struct {
	buckets         []string
	err             error
	projectID       string
	appID           string
	bucketName      string
	region          string
	roleARN         string
	externalID      *string
	allowedPrefixes []string
}

func (s *stubBucketConnectionsRepo) ListActiveBucketsForConnectionScope(_ context.Context, projectID string, appID string) ([]string, error) {
	s.projectID = projectID
	s.appID = appID
	if s.err != nil {
		return nil, s.err
	}
	return s.buckets, nil
}

func (s *stubBucketConnectionsRepo) CreateBucketConnection(_ context.Context, projectID string, appID string, bucketName string, region string, roleARN string, externalID *string, allowedPrefixes []string) error {
	s.projectID = projectID
	s.appID = appID
	s.bucketName = bucketName
	s.region = region
	s.roleARN = roleARN
	s.externalID = externalID
	s.allowedPrefixes = allowedPrefixes
	if s.err != nil {
		return s.err
	}
	return nil
}

func TestBucketConnectionsService_ListForScope(t *testing.T) {
	t.Run("forwards scope and returns buckets", func(t *testing.T) {
		repo := &stubBucketConnectionsRepo{buckets: []string{"bucket-a", "bucket-b"}}
		svc := NewBucketConnectionsService(repo)

		got, err := svc.ListForScope(context.Background(), "project-1", "app-1")
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}
		if repo.projectID != "project-1" || repo.appID != "app-1" {
			t.Fatalf("expected scope project-1/app-1, got %s/%s", repo.projectID, repo.appID)
		}
		if len(got) != 2 || got[0] != "bucket-a" || got[1] != "bucket-b" {
			t.Fatalf("unexpected buckets: %+v", got)
		}
	})

	t.Run("returns repository error", func(t *testing.T) {
		repo := &stubBucketConnectionsRepo{err: errors.New("repo failed")}
		svc := NewBucketConnectionsService(repo)

		_, err := svc.ListForScope(context.Background(), "project-1", "app-1")
		if err == nil {
			t.Fatal("expected error")
		}
	})
}

func TestBucketConnectionsService_CreateForScope(t *testing.T) {
	t.Run("forwards metadata to repository", func(t *testing.T) {
		externalID := "external-1"
		repo := &stubBucketConnectionsRepo{}
		svc := NewBucketConnectionsService(repo)

		err := svc.CreateForScope(
			context.Background(),
			"project-1",
			"app-1",
			"bucket-a",
			"us-east-1",
			"arn:aws:iam::123456789012:role/s3-service",
			&externalID,
			[]string{"uploads/", "avatars/"},
		)
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}
		if repo.projectID != "project-1" || repo.appID != "app-1" {
			t.Fatalf("expected scope project-1/app-1, got %s/%s", repo.projectID, repo.appID)
		}
		if repo.bucketName != "bucket-a" || repo.region != "us-east-1" || repo.roleARN == "" {
			t.Fatalf("unexpected create args: bucket=%s region=%s role=%s", repo.bucketName, repo.region, repo.roleARN)
		}
		if repo.externalID == nil || *repo.externalID != "external-1" {
			t.Fatalf("unexpected external id: %+v", repo.externalID)
		}
		if len(repo.allowedPrefixes) != 2 || repo.allowedPrefixes[0] != "uploads/" || repo.allowedPrefixes[1] != "avatars/" {
			t.Fatalf("unexpected allowed prefixes: %+v", repo.allowedPrefixes)
		}
	})

	t.Run("returns validation error for missing required fields", func(t *testing.T) {
		repo := &stubBucketConnectionsRepo{}
		svc := NewBucketConnectionsService(repo)

		err := svc.CreateForScope(context.Background(), "", "app-1", "bucket-a", "us-east-1", "arn:aws:iam::123456789012:role/s3-service", nil, nil)
		if err == nil {
			t.Fatal("expected validation error")
		}
		if !errors.Is(err, ErrInvalidBucketConnectionInput) {
			t.Fatalf("expected ErrInvalidBucketConnectionInput, got %v", err)
		}
	})

	t.Run("returns repository error", func(t *testing.T) {
		repo := &stubBucketConnectionsRepo{err: errors.New("repo failed")}
		svc := NewBucketConnectionsService(repo)

		err := svc.CreateForScope(context.Background(), "project-1", "app-1", "bucket-a", "us-east-1", "arn:aws:iam::123456789012:role/s3-service", nil, nil)
		if err == nil {
			t.Fatal("expected error")
		}
	})
}
