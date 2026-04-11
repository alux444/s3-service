package adapters

import (
	"context"
	"errors"
	"testing"
	"time"

	"s3-service/internal/s3"
	"s3-service/internal/service"
)

type stubListerHelper struct {
	input   s3.ListObjectsInput
	objects []s3.ListedObject
	err     error
}

func (s *stubListerHelper) ListObjects(_ context.Context, input s3.ListObjectsInput) ([]s3.ListedObject, error) {
	s.input = input
	if s.err != nil {
		return nil, s.err
	}
	return s.objects, nil
}

func TestS3ObjectListerAdapter_ListObjects(t *testing.T) {
	t.Run("maps input and output", func(t *testing.T) {
		now := time.Date(2026, time.April, 11, 12, 0, 0, 0, time.UTC)
		helper := &stubListerHelper{objects: []s3.ListedObject{{ObjectKey: "images/a.jpg", Size: 100, ETag: "etag-a", LastModified: now}}}
		adapter := &S3ObjectListerAdapter{helper: helper}
		externalID := "ext-1"

		objects, err := adapter.ListObjects(context.Background(), service.ObjectListRequest{
			BucketName: "bucket-a",
			Prefix:     "images/",
			Region:     "us-east-1",
			RoleARN:    "arn:aws:iam::123:role/s3",
			ExternalID: &externalID,
		})
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}
		if helper.input.BucketName != "bucket-a" || helper.input.Prefix != "images/" {
			t.Fatalf("unexpected helper input: %+v", helper.input)
		}
		if len(objects) != 1 || objects[0].ObjectKey != "images/a.jpg" || objects[0].Size != 100 || objects[0].ETag != "etag-a" || !objects[0].LastModified.Equal(now) {
			t.Fatalf("unexpected objects: %+v", objects)
		}
	})

	t.Run("returns helper error", func(t *testing.T) {
		expected := errors.New("list failed")
		adapter := &S3ObjectListerAdapter{helper: &stubListerHelper{err: expected}}

		_, err := adapter.ListObjects(context.Background(), service.ObjectListRequest{})
		if !errors.Is(err, expected) {
			t.Fatalf("expected wrapped error %v, got %v", expected, err)
		}
	})
}
