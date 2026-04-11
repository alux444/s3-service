package service

import (
	"context"
	"errors"
	"io"
	"strings"
	"testing"

	"s3-service/internal/database"
)

type stubObjectReadBucketRepo struct {
	buckets   []database.BucketConnection
	err       error
	projectID string
	appID     string
}

func (s *stubObjectReadBucketRepo) ListActiveBucketsForConnectionScope(_ context.Context, projectID string, appID string) ([]database.BucketConnection, error) {
	s.projectID = projectID
	s.appID = appID
	if s.err != nil {
		return nil, s.err
	}
	return s.buckets, nil
}

type stubObjectReader struct {
	input  ObjectReadInput
	result ObjectReadResult
	err    error
}

func (s *stubObjectReader) ReadObject(_ context.Context, input ObjectReadInput) (ObjectReadResult, error) {
	s.input = input
	if s.err != nil {
		return ObjectReadResult{}, s.err
	}
	return s.result, nil
}

func TestObjectReadService_ReadObject(t *testing.T) {
	t.Run("resolves bucket connection metadata and reads", func(t *testing.T) {
		externalID := "ext-1"
		repo := &stubObjectReadBucketRepo{buckets: []database.BucketConnection{{BucketName: "bucket-a", Region: "us-east-1", RoleARN: "arn:aws:iam::123456789012:role/s3", ExternalID: &externalID, AllowedPrefixes: []string{"uploads/"}}}}
		reader := &stubObjectReader{result: ObjectReadResult{Body: io.NopCloser(strings.NewReader("payload")), ContentType: "image/jpeg", ContentLength: 7, ETag: "etag-1"}}
		svc := NewObjectReadService(repo, reader)

		result, err := svc.ReadObject(context.Background(), ObjectReadInput{
			ProjectID:  "project-1",
			AppID:      "app-1",
			BucketName: "bucket-a",
			ObjectKey:  "uploads/a.jpg",
		})
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}
		if repo.projectID != "project-1" || repo.appID != "app-1" {
			t.Fatalf("expected scope project-1/app-1, got %s/%s", repo.projectID, repo.appID)
		}
		if reader.input.Region != "us-east-1" || reader.input.RoleARN == "" {
			t.Fatalf("expected region/role populated, got region=%s role=%s", reader.input.Region, reader.input.RoleARN)
		}
		if len(reader.input.AllowedPrefixes) != 1 || reader.input.AllowedPrefixes[0] != "uploads/" {
			t.Fatalf("unexpected allowed prefixes: %+v", reader.input.AllowedPrefixes)
		}
		if result.ContentType != "image/jpeg" || result.ContentLength != 7 || result.ETag != "etag-1" {
			t.Fatalf("unexpected result: %+v", result)
		}
	})

	t.Run("returns validation error for missing required fields", func(t *testing.T) {
		svc := NewObjectReadService(&stubObjectReadBucketRepo{}, &stubObjectReader{})

		_, err := svc.ReadObject(context.Background(), ObjectReadInput{BucketName: "bucket-a", ObjectKey: "uploads/a.jpg"})
		if err == nil {
			t.Fatal("expected validation error")
		}
		if !errors.Is(err, ErrInvalidObjectReadInput) {
			t.Fatalf("expected ErrInvalidObjectReadInput, got %v", err)
		}
	})

	t.Run("returns bucket connection not found", func(t *testing.T) {
		svc := NewObjectReadService(&stubObjectReadBucketRepo{buckets: []database.BucketConnection{}}, &stubObjectReader{})

		_, err := svc.ReadObject(context.Background(), ObjectReadInput{ProjectID: "project-1", AppID: "app-1", BucketName: "bucket-a", ObjectKey: "uploads/a.jpg"})
		if err == nil {
			t.Fatal("expected error")
		}
		if !errors.Is(err, ErrBucketConnectionNotFound) {
			t.Fatalf("expected ErrBucketConnectionNotFound, got %v", err)
		}
	})

	t.Run("returns repository error", func(t *testing.T) {
		svc := NewObjectReadService(&stubObjectReadBucketRepo{err: errors.New("db down")}, &stubObjectReader{})

		_, err := svc.ReadObject(context.Background(), ObjectReadInput{ProjectID: "project-1", AppID: "app-1", BucketName: "bucket-a", ObjectKey: "uploads/a.jpg"})
		if err == nil {
			t.Fatal("expected error")
		}
	})
}
