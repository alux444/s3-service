package service

import (
	"context"
	"errors"
	"testing"

	"s3-service/internal/database"
)

type stubObjectUploadBucketRepo struct {
	buckets   []database.BucketConnection
	err       error
	projectID string
	appID     string
}

func (s *stubObjectUploadBucketRepo) ListActiveBucketsForConnectionScope(_ context.Context, projectID string, appID string) ([]database.BucketConnection, error) {
	s.projectID = projectID
	s.appID = appID
	if s.err != nil {
		return nil, s.err
	}
	return s.buckets, nil
}

type stubObjectUploader struct {
	input  ObjectUploadInput
	result ObjectUploadResult
	err    error
}

func (s *stubObjectUploader) UploadObject(_ context.Context, input ObjectUploadInput) (ObjectUploadResult, error) {
	s.input = input
	if s.err != nil {
		return ObjectUploadResult{}, s.err
	}
	return s.result, nil
}

func TestObjectUploadService_UploadObject(t *testing.T) {
	t.Run("resolves bucket connection metadata and uploads", func(t *testing.T) {
		externalID := "ext-1"
		repo := &stubObjectUploadBucketRepo{buckets: []database.BucketConnection{{BucketName: "bucket-a", Region: "us-east-1", RoleARN: "arn:aws:iam::123456789012:role/s3", ExternalID: &externalID}}}
		uploader := &stubObjectUploader{result: ObjectUploadResult{ETag: "etag-1", Size: 7}}
		svc := NewObjectUploadService(repo, uploader)

		result, err := svc.UploadObject(context.Background(), ObjectUploadInput{
			ProjectID:   "project-1",
			AppID:       "app-1",
			BucketName:  "bucket-a",
			ObjectKey:   "uploads/a.jpg",
			ContentType: "image/jpeg",
			Body:        []byte("payload"),
		})
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}
		if repo.projectID != "project-1" || repo.appID != "app-1" {
			t.Fatalf("expected scope project-1/app-1, got %s/%s", repo.projectID, repo.appID)
		}
		if uploader.input.Region != "us-east-1" || uploader.input.RoleARN == "" {
			t.Fatalf("expected region/role to be populated, got region=%s role=%s", uploader.input.Region, uploader.input.RoleARN)
		}
		if uploader.input.ExternalID == nil || *uploader.input.ExternalID != "ext-1" {
			t.Fatalf("expected external id ext-1, got %+v", uploader.input.ExternalID)
		}
		if result.ETag != "etag-1" || result.Size != 7 {
			t.Fatalf("unexpected result: %+v", result)
		}
	})

	t.Run("returns validation error for missing required fields", func(t *testing.T) {
		svc := NewObjectUploadService(&stubObjectUploadBucketRepo{}, &stubObjectUploader{})

		_, err := svc.UploadObject(context.Background(), ObjectUploadInput{BucketName: "bucket-a", ObjectKey: "uploads/a.jpg", Body: []byte("payload")})
		if err == nil {
			t.Fatal("expected validation error")
		}
		if !errors.Is(err, ErrInvalidObjectUploadInput) {
			t.Fatalf("expected ErrInvalidObjectUploadInput, got %v", err)
		}
	})

	t.Run("returns bucket connection not found", func(t *testing.T) {
		svc := NewObjectUploadService(&stubObjectUploadBucketRepo{buckets: []database.BucketConnection{}}, &stubObjectUploader{})

		_, err := svc.UploadObject(context.Background(), ObjectUploadInput{ProjectID: "project-1", AppID: "app-1", BucketName: "bucket-a", ObjectKey: "uploads/a.jpg", Body: []byte("payload")})
		if err == nil {
			t.Fatal("expected error")
		}
		if !errors.Is(err, ErrBucketConnectionNotFound) {
			t.Fatalf("expected ErrBucketConnectionNotFound, got %v", err)
		}
	})

	t.Run("returns repository error", func(t *testing.T) {
		svc := NewObjectUploadService(&stubObjectUploadBucketRepo{err: errors.New("db down")}, &stubObjectUploader{})

		_, err := svc.UploadObject(context.Background(), ObjectUploadInput{ProjectID: "project-1", AppID: "app-1", BucketName: "bucket-a", ObjectKey: "uploads/a.jpg", Body: []byte("payload")})
		if err == nil {
			t.Fatal("expected error")
		}
	})
}
