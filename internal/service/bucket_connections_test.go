package service

import (
	"context"
	"errors"
	"testing"

	"s3-service/internal/database"
)

type stubBucketSecurityValidator struct {
	err        error
	bucketName string
	region     string
	roleARN    string
	externalID *string
}

func (s *stubBucketSecurityValidator) ValidateBucketConnection(_ context.Context, bucketName string, region string, roleARN string, externalID *string) error {
	s.bucketName = bucketName
	s.region = region
	s.roleARN = roleARN
	s.externalID = externalID
	return s.err
}

type stubBucketConnectionsRepo struct {
	buckets         []database.BucketConnection
	err             error
	projectID       string
	appID           string
	bucketName      string
	region          string
	roleARN         string
	externalID      *string
	allowedPrefixes []string
}

func (s *stubBucketConnectionsRepo) ListActiveBucketsForConnectionScope(_ context.Context, projectID string, appID string) ([]database.BucketConnection, error) {
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
		repo := &stubBucketConnectionsRepo{buckets: []database.BucketConnection{{BucketName: "bucket-a", Region: "us-east-1", RoleARN: "arn:aws:iam::123456789012:role/s3-a"}, {BucketName: "bucket-b", Region: "us-west-2", RoleARN: "arn:aws:iam::123456789012:role/s3-b"}}}
		svc := NewBucketConnectionsService(repo)

		got, err := svc.ListForScope(context.Background(), "project-1", "app-1")
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}
		if repo.projectID != "project-1" || repo.appID != "app-1" {
			t.Fatalf("expected scope project-1/app-1, got %s/%s", repo.projectID, repo.appID)
		}
		if len(got) != 2 || got[0].BucketName != "bucket-a" || got[1].BucketName != "bucket-b" {
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

	t.Run("runs bucket security validator before save", func(t *testing.T) {
		externalID := "external-1"
		repo := &stubBucketConnectionsRepo{}
		validator := &stubBucketSecurityValidator{}
		svc := NewBucketConnectionsService(repo, WithBucketConnectionSecurityValidator(validator))

		err := svc.CreateForScope(context.Background(), "project-1", "app-1", "bucket-a", "us-east-1", "arn:aws:iam::123456789012:role/s3-service", &externalID, nil)
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}
		if validator.bucketName != "bucket-a" || validator.region != "us-east-1" || validator.roleARN == "" {
			t.Fatalf("unexpected validator args: bucket=%s region=%s role=%s", validator.bucketName, validator.region, validator.roleARN)
		}
		if repo.bucketName != "bucket-a" {
			t.Fatalf("expected repo create call, got bucket=%s", repo.bucketName)
		}
	})

	t.Run("returns validator error and skips save", func(t *testing.T) {
		repo := &stubBucketConnectionsRepo{}
		validator := &stubBucketSecurityValidator{err: errors.New("bucket security baseline violation")}
		svc := NewBucketConnectionsService(repo, WithBucketConnectionSecurityValidator(validator))

		err := svc.CreateForScope(context.Background(), "project-1", "app-1", "bucket-a", "us-east-1", "arn:aws:iam::123456789012:role/s3-service", nil, nil)
		if err == nil {
			t.Fatal("expected error")
		}
		if repo.bucketName != "" {
			t.Fatalf("expected repository create not called, got bucket=%s", repo.bucketName)
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
