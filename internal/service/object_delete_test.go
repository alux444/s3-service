package service

import (
	"context"
	"errors"
	"testing"

	"s3-service/internal/database"
)

type stubObjectDeleteBucketRepo struct {
	buckets   []database.BucketConnection
	err       error
	projectID string
	appID     string
}

func (s *stubObjectDeleteBucketRepo) ListActiveBucketsForConnectionScope(_ context.Context, projectID string, appID string) ([]database.BucketConnection, error) {
	s.projectID = projectID
	s.appID = appID
	if s.err != nil {
		return nil, s.err
	}
	return s.buckets, nil
}

type stubObjectDeleter struct {
	input  ObjectDeleteInput
	result ObjectDeleteResult
	err    error
}

func (s *stubObjectDeleter) DeleteObject(_ context.Context, input ObjectDeleteInput) (ObjectDeleteResult, error) {
	s.input = input
	if s.err != nil {
		return ObjectDeleteResult{}, s.err
	}
	return s.result, nil
}

func TestObjectDeleteService_DeleteObject(t *testing.T) {
	t.Run("resolves bucket connection metadata and deletes", func(t *testing.T) {
		externalID := "ext-1"
		repo := &stubObjectDeleteBucketRepo{buckets: []database.BucketConnection{{BucketName: "bucket-a", Region: "us-east-1", RoleARN: "arn:aws:iam::123456789012:role/s3", ExternalID: &externalID, AllowedPrefixes: []string{"uploads/"}}}}
		deleter := &stubObjectDeleter{result: ObjectDeleteResult{Deleted: true}}
		svc := NewObjectDeleteService(repo, deleter)

		result, err := svc.DeleteObject(context.Background(), ObjectDeleteInput{
			ProjectID:  "project-1",
			AppID:      "app-1",
			BucketName: "bucket-a",
			ObjectKey:  "uploads/a.jpg",
		})
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}
		if !result.Deleted {
			t.Fatal("expected deleted=true")
		}
		if repo.projectID != "project-1" || repo.appID != "app-1" {
			t.Fatalf("expected scope project-1/app-1, got %s/%s", repo.projectID, repo.appID)
		}
		if deleter.input.Region != "us-east-1" || deleter.input.RoleARN == "" {
			t.Fatalf("expected region/role populated, got region=%s role=%s", deleter.input.Region, deleter.input.RoleARN)
		}
		if len(deleter.input.AllowedPrefixes) != 1 || deleter.input.AllowedPrefixes[0] != "uploads/" {
			t.Fatalf("unexpected allowed prefixes: %+v", deleter.input.AllowedPrefixes)
		}
	})

	t.Run("returns validation error for missing required fields", func(t *testing.T) {
		svc := NewObjectDeleteService(&stubObjectDeleteBucketRepo{}, &stubObjectDeleter{})

		_, err := svc.DeleteObject(context.Background(), ObjectDeleteInput{BucketName: "bucket-a", ObjectKey: "uploads/a.jpg"})
		if err == nil {
			t.Fatal("expected validation error")
		}
		if !errors.Is(err, ErrInvalidObjectDeleteInput) {
			t.Fatalf("expected ErrInvalidObjectDeleteInput, got %v", err)
		}
	})

	t.Run("returns bucket connection not found", func(t *testing.T) {
		svc := NewObjectDeleteService(&stubObjectDeleteBucketRepo{buckets: []database.BucketConnection{}}, &stubObjectDeleter{})

		_, err := svc.DeleteObject(context.Background(), ObjectDeleteInput{ProjectID: "project-1", AppID: "app-1", BucketName: "bucket-a", ObjectKey: "uploads/a.jpg"})
		if err == nil {
			t.Fatal("expected error")
		}
		if !errors.Is(err, ErrBucketConnectionNotFound) {
			t.Fatalf("expected ErrBucketConnectionNotFound, got %v", err)
		}
	})

	t.Run("returns repository error", func(t *testing.T) {
		svc := NewObjectDeleteService(&stubObjectDeleteBucketRepo{err: errors.New("db down")}, &stubObjectDeleter{})

		_, err := svc.DeleteObject(context.Background(), ObjectDeleteInput{ProjectID: "project-1", AppID: "app-1", BucketName: "bucket-a", ObjectKey: "uploads/a.jpg"})
		if err == nil {
			t.Fatal("expected error")
		}
	})
}
