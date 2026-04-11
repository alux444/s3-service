package adapters

import (
	"context"
	"errors"
	"io"
	"strings"
	"testing"
	"time"

	"s3-service/internal/s3"
	"s3-service/internal/service"
)

type stubUploaderHelper struct {
	input  s3.UploadObjectInput
	result s3.UploadObjectResult
	err    error
}

func (s *stubUploaderHelper) UploadObject(_ context.Context, input s3.UploadObjectInput) (s3.UploadObjectResult, error) {
	s.input = input
	if s.err != nil {
		return s3.UploadObjectResult{}, s.err
	}
	return s.result, nil
}

type stubDeleterHelper struct {
	input  s3.DeleteObjectInput
	result s3.DeleteObjectResult
	err    error
}

func (s *stubDeleterHelper) DeleteObject(_ context.Context, input s3.DeleteObjectInput) (s3.DeleteObjectResult, error) {
	s.input = input
	if s.err != nil {
		return s3.DeleteObjectResult{}, s.err
	}
	return s.result, nil
}

type stubPresignerHelper struct {
	input  s3.PresignObjectInput
	result s3.PresignObjectResult
	err    error
}

type stubReaderHelper struct {
	input  s3.GetObjectInput
	result s3.GetObjectResult
	err    error
}

func (s *stubPresignerHelper) PresignObject(_ context.Context, input s3.PresignObjectInput) (s3.PresignObjectResult, error) {
	s.input = input
	if s.err != nil {
		return s3.PresignObjectResult{}, s.err
	}
	return s.result, nil
}

func (s *stubReaderHelper) GetObject(_ context.Context, input s3.GetObjectInput) (s3.GetObjectResult, error) {
	s.input = input
	if s.err != nil {
		return s3.GetObjectResult{}, s.err
	}
	return s.result, nil
}

func TestS3ObjectUploaderAdapter_UploadObject(t *testing.T) {
	t.Run("maps input output", func(t *testing.T) {
		help := &stubUploaderHelper{result: s3.UploadObjectResult{ETag: "etag-1", Size: 7}}
		adapter := &S3ObjectUploaderAdapter{helper: help}
		externalID := "ext-1"

		result, err := adapter.UploadObject(context.Background(), service.ObjectUploadInput{
			BucketName:  "bucket-a",
			ObjectKey:   "uploads/a.jpg",
			Region:      "us-east-1",
			RoleARN:     "arn:aws:iam::123:role/test",
			ExternalID:  &externalID,
			ContentType: "image/jpeg",
			Body:        []byte("payload"),
			Metadata:    map[string]string{"k": "v"},
		})
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}
		if help.input.BucketName != "bucket-a" || help.input.ObjectKey != "uploads/a.jpg" {
			t.Fatalf("unexpected helper input: %+v", help.input)
		}
		if result.ETag != "etag-1" || result.Size != 7 {
			t.Fatalf("unexpected result: %+v", result)
		}
	})

	t.Run("returns helper error", func(t *testing.T) {
		expected := errors.New("upload failed")
		adapter := &S3ObjectUploaderAdapter{helper: &stubUploaderHelper{err: expected}}

		_, err := adapter.UploadObject(context.Background(), service.ObjectUploadInput{})
		if !errors.Is(err, expected) {
			t.Fatalf("expected wrapped error %v, got %v", expected, err)
		}
	})
}

func TestS3ObjectDeleterAdapter_DeleteObject(t *testing.T) {
	t.Run("maps input output", func(t *testing.T) {
		help := &stubDeleterHelper{result: s3.DeleteObjectResult{Deleted: true}}
		adapter := &S3ObjectDeleterAdapter{helper: help}
		externalID := "ext-1"

		result, err := adapter.DeleteObject(context.Background(), service.ObjectDeleteInput{
			BucketName:      "bucket-a",
			ObjectKey:       "uploads/a.jpg",
			Region:          "us-east-1",
			RoleARN:         "arn:aws:iam::123:role/test",
			ExternalID:      &externalID,
			AllowedPrefixes: []string{"uploads/"},
		})
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}
		if help.input.BucketName != "bucket-a" || len(help.input.AllowedPrefixes) != 1 {
			t.Fatalf("unexpected helper input: %+v", help.input)
		}
		if !result.Deleted {
			t.Fatalf("expected deleted result, got %+v", result)
		}
	})

	t.Run("returns helper error", func(t *testing.T) {
		expected := errors.New("delete failed")
		adapter := &S3ObjectDeleterAdapter{helper: &stubDeleterHelper{err: expected}}

		_, err := adapter.DeleteObject(context.Background(), service.ObjectDeleteInput{})
		if !errors.Is(err, expected) {
			t.Fatalf("expected wrapped error %v, got %v", expected, err)
		}
	})
}

func TestS3ObjectPresignerAdapter_PresignObject(t *testing.T) {
	t.Run("maps input output", func(t *testing.T) {
		help := &stubPresignerHelper{result: s3.PresignObjectResult{URL: "https://example.test/signed", Method: "PUT", ExpiresIn: 60 * time.Second}}
		adapter := &S3ObjectPresignerAdapter{helper: help}
		externalID := "ext-1"

		result, err := adapter.PresignObject(context.Background(), service.ObjectPresignInput{
			BucketName:  "bucket-a",
			ObjectKey:   "uploads/a.jpg",
			Region:      "us-east-1",
			RoleARN:     "arn:aws:iam::123:role/test",
			ExternalID:  &externalID,
			Method:      "PUT",
			ExpiresIn:   60 * time.Second,
			ContentType: "image/jpeg",
		})
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}
		if help.input.Method != "PUT" || help.input.ContentType != "image/jpeg" {
			t.Fatalf("unexpected helper input: %+v", help.input)
		}
		if result.URL == "" || result.Method != "PUT" || result.ExpiresIn != 60*time.Second {
			t.Fatalf("unexpected result: %+v", result)
		}
	})

	t.Run("returns helper error", func(t *testing.T) {
		expected := errors.New("presign failed")
		adapter := &S3ObjectPresignerAdapter{helper: &stubPresignerHelper{err: expected}}

		_, err := adapter.PresignObject(context.Background(), service.ObjectPresignInput{})
		if !errors.Is(err, expected) {
			t.Fatalf("expected wrapped error %v, got %v", expected, err)
		}
	})
}

func TestS3ObjectReaderAdapter_ReadObject(t *testing.T) {
	t.Run("maps input output", func(t *testing.T) {
		help := &stubReaderHelper{result: s3.GetObjectResult{Body: io.NopCloser(strings.NewReader("payload")), ContentType: "image/jpeg", ContentLength: 7, ETag: "etag-1"}}
		adapter := &S3ObjectReaderAdapter{helper: help}
		externalID := "ext-1"

		result, err := adapter.ReadObject(context.Background(), service.ObjectReadInput{
			BucketName:      "bucket-a",
			ObjectKey:       "uploads/a.jpg",
			Region:          "us-east-1",
			RoleARN:         "arn:aws:iam::123:role/test",
			ExternalID:      &externalID,
			AllowedPrefixes: []string{"uploads/"},
		})
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}
		if help.input.BucketName != "bucket-a" || help.input.ObjectKey != "uploads/a.jpg" {
			t.Fatalf("unexpected helper input: %+v", help.input)
		}
		if result.ContentType != "image/jpeg" || result.ContentLength != 7 || result.ETag != "etag-1" {
			t.Fatalf("unexpected result metadata: %+v", result)
		}
		body, readErr := io.ReadAll(result.Body)
		if readErr != nil {
			t.Fatalf("failed to read body: %v", readErr)
		}
		if string(body) != "payload" {
			t.Fatalf("unexpected body: %q", string(body))
		}
	})

	t.Run("returns helper error", func(t *testing.T) {
		expected := errors.New("read failed")
		adapter := &S3ObjectReaderAdapter{helper: &stubReaderHelper{err: expected}}

		_, err := adapter.ReadObject(context.Background(), service.ObjectReadInput{})
		if !errors.Is(err, expected) {
			t.Fatalf("expected wrapped error %v, got %v", expected, err)
		}
	})
}
