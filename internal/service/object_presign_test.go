package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"s3-service/internal/database"
)

type stubObjectPresigner struct {
	input  ObjectPresignInput
	result ObjectPresignResult
	err    error
}

func (s *stubObjectPresigner) PresignObject(_ context.Context, input ObjectPresignInput) (ObjectPresignResult, error) {
	s.input = input
	if s.err != nil {
		return ObjectPresignResult{}, s.err
	}
	return s.result, nil
}

func TestObjectPresignService_PresignObject(t *testing.T) {
	t.Run("resolves bucket metadata and presigns", func(t *testing.T) {
		externalID := "ext-1"
		repo := &stubObjectUploadBucketRepo{buckets: []database.BucketConnection{{
			BucketName: "bucket-a",
			Region:     "us-east-1",
			RoleARN:    "arn:aws:iam::123456789012:role/s3",
			ExternalID: &externalID,
		}}}
		presigner := &stubObjectPresigner{result: ObjectPresignResult{URL: "https://example.test/u", Method: PresignMethodGet, ExpiresIn: 75 * time.Second}}
		svc := NewObjectPresignService(repo, presigner)

		result, err := svc.PresignObject(context.Background(), ObjectPresignInput{
			ProjectID:  "project-1",
			AppID:      "app-1",
			BucketName: "bucket-a",
			ObjectKey:  "uploads/a.jpg",
			Method:     "get",
			ExpiresIn:  75 * time.Second,
		})
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}
		if repo.projectID != "project-1" || repo.appID != "app-1" {
			t.Fatalf("expected scope project-1/app-1, got %s/%s", repo.projectID, repo.appID)
		}
		if presigner.input.Method != PresignMethodGet {
			t.Fatalf("expected method GET, got %q", presigner.input.Method)
		}
		if presigner.input.Region != "us-east-1" || presigner.input.RoleARN == "" {
			t.Fatalf("expected role metadata to be populated, got region=%s role=%s", presigner.input.Region, presigner.input.RoleARN)
		}
		if presigner.input.ExternalID == nil || *presigner.input.ExternalID != "ext-1" {
			t.Fatalf("expected external id ext-1, got %+v", presigner.input.ExternalID)
		}
		if result.URL == "" || result.Method != PresignMethodGet {
			t.Fatalf("unexpected result: %+v", result)
		}
	})

	t.Run("requires content type for put", func(t *testing.T) {
		svc := NewObjectPresignService(&stubObjectUploadBucketRepo{}, &stubObjectPresigner{})

		_, err := svc.PresignObject(context.Background(), ObjectPresignInput{
			ProjectID:  "project-1",
			AppID:      "app-1",
			BucketName: "bucket-a",
			ObjectKey:  "uploads/a.jpg",
			Method:     PresignMethodPut,
		})
		if err == nil || !errors.Is(err, ErrInvalidObjectPresignInput) {
			t.Fatalf("expected ErrInvalidObjectPresignInput, got %v", err)
		}
	})

	t.Run("returns bucket connection not found", func(t *testing.T) {
		svc := NewObjectPresignService(&stubObjectUploadBucketRepo{buckets: []database.BucketConnection{}}, &stubObjectPresigner{})

		_, err := svc.PresignObject(context.Background(), ObjectPresignInput{
			ProjectID:  "project-1",
			AppID:      "app-1",
			BucketName: "bucket-a",
			ObjectKey:  "uploads/a.jpg",
			Method:     PresignMethodGet,
		})
		if err == nil || !errors.Is(err, ErrBucketConnectionNotFound) {
			t.Fatalf("expected ErrBucketConnectionNotFound, got %v", err)
		}
	})

	t.Run("normalizes ttl for put", func(t *testing.T) {
		repo := &stubObjectUploadBucketRepo{buckets: []database.BucketConnection{{BucketName: "bucket-a", Region: "us-east-1", RoleARN: "arn:aws:iam::123456789012:role/s3"}}}
		presigner := &stubObjectPresigner{result: ObjectPresignResult{URL: "https://example.test/u", Method: PresignMethodPut, ExpiresIn: MaxPresignPutTTL}}
		svc := NewObjectPresignService(repo, presigner)

		_, err := svc.PresignObject(context.Background(), ObjectPresignInput{
			ProjectID:   "project-1",
			AppID:       "app-1",
			BucketName:  "bucket-a",
			ObjectKey:   "uploads/a.jpg",
			Method:      PresignMethodPut,
			ContentType: "image/jpeg",
			ExpiresIn:   30 * time.Minute,
		})
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}
		if presigner.input.ExpiresIn != MaxPresignPutTTL {
			t.Fatalf("expected ttl %s, got %s", MaxPresignPutTTL, presigner.input.ExpiresIn)
		}
	})
}
