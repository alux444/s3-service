package s3

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	awss3 "github.com/aws/aws-sdk-go-v2/service/s3"
	awss3types "github.com/aws/aws-sdk-go-v2/service/s3/types"
	"github.com/aws/smithy-go"
)

type stubListRoleConfigProvider struct {
	cfg aws.Config
	err error
	ref BucketRoleReference
}

func (s *stubListRoleConfigProvider) ConfigForRole(_ context.Context, ref BucketRoleReference) (aws.Config, error) {
	s.ref = ref
	if s.err != nil {
		return aws.Config{}, s.err
	}
	return s.cfg, nil
}

type stubListClient struct {
	outs  []*awss3.ListObjectsV2Output
	err   error
	errs  []error
	calls int
	input *awss3.ListObjectsV2Input
}

func (s *stubListClient) ListObjectsV2(_ context.Context, params *awss3.ListObjectsV2Input, _ ...func(*awss3.Options)) (*awss3.ListObjectsV2Output, error) {
	s.calls++
	s.input = params
	if len(s.errs) > 0 {
		err := s.errs[0]
		s.errs = s.errs[1:]
		if err != nil {
			return nil, err
		}
	}
	if s.err != nil {
		return nil, s.err
	}
	if len(s.outs) == 0 {
		return &awss3.ListObjectsV2Output{}, nil
	}
	out := s.outs[0]
	s.outs = s.outs[1:]
	return out, nil
}

type testListAPIError struct {
	code string
	msg  string
}

func (e testListAPIError) ErrorCode() string             { return e.code }
func (e testListAPIError) ErrorMessage() string          { return e.msg }
func (e testListAPIError) ErrorFault() smithy.ErrorFault { return smithy.FaultClient }
func (e testListAPIError) Error() string                 { return e.code + ": " + e.msg }

func TestListHelper_ListObjects(t *testing.T) {
	t.Run("lists objects with pagination", func(t *testing.T) {
		now := time.Date(2026, time.April, 11, 10, 0, 0, 0, time.UTC)
		provider := &stubListRoleConfigProvider{cfg: aws.Config{Region: "us-east-1"}}
		client := &stubListClient{outs: []*awss3.ListObjectsV2Output{
			{
				Contents:              []awss3types.Object{{Key: aws.String("images/a.jpg"), Size: aws.Int64(100), ETag: aws.String("etag-a"), LastModified: aws.Time(now)}},
				IsTruncated:           aws.Bool(true),
				NextContinuationToken: aws.String("next-token"),
			},
			{
				Contents:    []awss3types.Object{{Key: aws.String("images/b.jpg"), Size: aws.Int64(120), ETag: aws.String("etag-b"), LastModified: aws.Time(now.Add(1 * time.Minute))}},
				IsTruncated: aws.Bool(false),
			},
		}}
		helper := &ListHelper{
			cache:       provider,
			maxObjects:  200,
			retryPolicy: retryPolicy{maxAttempts: 2, sleep: func(context.Context, time.Duration) error { return nil }},
			clientFactory: func(aws.Config) listClient {
				return client
			},
		}

		objects, err := helper.ListObjects(context.Background(), ListObjectsInput{BucketName: "bucket-a", Prefix: "images/", Region: "us-east-1", RoleARN: "arn:aws:iam::123:role/s3"})
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}
		if provider.ref.Region != "us-east-1" || provider.ref.RoleARN == "" {
			t.Fatalf("unexpected role ref: %+v", provider.ref)
		}
		if client.calls != 2 {
			t.Fatalf("expected 2 list calls, got %d", client.calls)
		}
		if len(objects) != 2 || objects[0].ObjectKey != "images/a.jpg" || objects[1].ObjectKey != "images/b.jpg" {
			t.Fatalf("unexpected list objects result: %+v", objects)
		}
	})

	t.Run("returns validation error for missing required fields", func(t *testing.T) {
		helper := &ListHelper{cache: &stubListRoleConfigProvider{}, clientFactory: func(aws.Config) listClient { return &stubListClient{} }}

		_, err := helper.ListObjects(context.Background(), ListObjectsInput{BucketName: "bucket-a"})
		if err == nil {
			t.Fatal("expected error")
		}
		if !errors.Is(err, ErrInvalidAssumeRoleInput) {
			t.Fatalf("expected ErrInvalidAssumeRoleInput, got %v", err)
		}
	})

	t.Run("retries on throttling and succeeds", func(t *testing.T) {
		client := &stubListClient{
			errs: []error{testListAPIError{code: "ThrottlingException", msg: "slow down"}, nil},
			outs: []*awss3.ListObjectsV2Output{{Contents: []awss3types.Object{{Key: aws.String("images/a.jpg")}}, IsTruncated: aws.Bool(false)}},
		}
		helper := &ListHelper{
			cache:       &stubListRoleConfigProvider{cfg: aws.Config{}},
			maxObjects:  10,
			retryPolicy: retryPolicy{maxAttempts: 3, sleep: func(context.Context, time.Duration) error { return nil }},
			clientFactory: func(aws.Config) listClient {
				return client
			},
		}

		objects, err := helper.ListObjects(context.Background(), ListObjectsInput{BucketName: "bucket-a", Prefix: "images/", Region: "us-east-1", RoleARN: "arn:aws:iam::123:role/s3"})
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}
		if len(objects) != 1 {
			t.Fatalf("expected one object, got %+v", objects)
		}
		if client.calls != 2 {
			t.Fatalf("expected 2 attempts, got %d", client.calls)
		}
	})

	t.Run("returns upstream list error", func(t *testing.T) {
		helper := &ListHelper{
			cache:       &stubListRoleConfigProvider{cfg: aws.Config{}},
			maxObjects:  10,
			retryPolicy: retryPolicy{maxAttempts: 2, sleep: func(context.Context, time.Duration) error { return nil }},
			clientFactory: func(aws.Config) listClient {
				return &stubListClient{err: errors.New("s3 timeout")}
			},
		}

		_, err := helper.ListObjects(context.Background(), ListObjectsInput{BucketName: "bucket-a", Prefix: "images/", Region: "us-east-1", RoleARN: "arn:aws:iam::123:role/s3"})
		if err == nil {
			t.Fatal("expected error")
		}
	})
}
