package s3

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	awss3 "github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/smithy-go"
)

type stubDeleteRoleConfigProvider struct {
	cfg aws.Config
	err error
	ref BucketRoleReference
}

func (s *stubDeleteRoleConfigProvider) ConfigForRole(_ context.Context, ref BucketRoleReference) (aws.Config, error) {
	s.ref = ref
	if s.err != nil {
		return aws.Config{}, s.err
	}
	return s.cfg, nil
}

type stubDeleteClient struct {
	err   error
	errs  []error
	input *awss3.DeleteObjectInput
	calls int
}

func (s *stubDeleteClient) DeleteObject(_ context.Context, params *awss3.DeleteObjectInput, _ ...func(*awss3.Options)) (*awss3.DeleteObjectOutput, error) {
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
	return &awss3.DeleteObjectOutput{}, nil
}

type testDeleteAPIError struct {
	code string
	msg  string
}

func (e testDeleteAPIError) ErrorCode() string             { return e.code }
func (e testDeleteAPIError) ErrorMessage() string          { return e.msg }
func (e testDeleteAPIError) ErrorFault() smithy.ErrorFault { return smithy.FaultClient }
func (e testDeleteAPIError) Error() string                 { return e.code + ": " + e.msg }

func TestDeleteHelper_DeleteObject(t *testing.T) {
	t.Run("deletes object when input is valid", func(t *testing.T) {
		provider := &stubDeleteRoleConfigProvider{cfg: aws.Config{Region: "us-east-1"}}
		client := &stubDeleteClient{}
		helper := &DeleteHelper{
			cache: provider,
			clientFactory: func(aws.Config) deleteClient {
				return client
			},
		}

		externalID := "ext-1"
		result, err := helper.DeleteObject(context.Background(), DeleteObjectInput{
			BucketName:      "bucket-a",
			ObjectKey:       "uploads/a.jpg",
			Region:          "us-east-1",
			RoleARN:         "arn:aws:iam::123456789012:role/s3",
			ExternalID:      &externalID,
			AllowedPrefixes: []string{"uploads/"},
		})
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}
		if !result.Deleted {
			t.Fatal("expected deleted=true")
		}
		if provider.ref.Region != "us-east-1" || provider.ref.RoleARN == "" {
			t.Fatalf("unexpected role ref: %+v", provider.ref)
		}
		if client.input == nil || aws.ToString(client.input.Key) != "uploads/a.jpg" {
			t.Fatalf("unexpected delete input: %+v", client.input)
		}
	})

	t.Run("returns guardrail error when key is outside allowed prefixes", func(t *testing.T) {
		helper := &DeleteHelper{cache: &stubDeleteRoleConfigProvider{cfg: aws.Config{}}, clientFactory: func(aws.Config) deleteClient { return &stubDeleteClient{} }}

		_, err := helper.DeleteObject(context.Background(), DeleteObjectInput{
			BucketName:      "bucket-a",
			ObjectKey:       "private/a.jpg",
			Region:          "us-east-1",
			RoleARN:         "arn:aws:iam::123456789012:role/s3",
			AllowedPrefixes: []string{"uploads/"},
		})
		if err == nil {
			t.Fatal("expected error")
		}
		if !errors.Is(err, ErrDeletePrefixGuardrailViolation) {
			t.Fatalf("expected ErrDeletePrefixGuardrailViolation, got %v", err)
		}
	})

	t.Run("returns guardrail error for unsafe key shape", func(t *testing.T) {
		helper := &DeleteHelper{cache: &stubDeleteRoleConfigProvider{cfg: aws.Config{}}, clientFactory: func(aws.Config) deleteClient { return &stubDeleteClient{} }}

		_, err := helper.DeleteObject(context.Background(), DeleteObjectInput{
			BucketName:      "bucket-a",
			ObjectKey:       "uploads/",
			Region:          "us-east-1",
			RoleARN:         "arn:aws:iam::123456789012:role/s3",
			AllowedPrefixes: []string{"uploads/"},
		})
		if err == nil {
			t.Fatal("expected error")
		}
		if !errors.Is(err, ErrDeletePrefixGuardrailViolation) {
			t.Fatalf("expected ErrDeletePrefixGuardrailViolation, got %v", err)
		}
	})

	t.Run("treats NoSuchKey as success for idempotency", func(t *testing.T) {
		helper := &DeleteHelper{
			cache: &stubDeleteRoleConfigProvider{cfg: aws.Config{}},
			clientFactory: func(aws.Config) deleteClient {
				return &stubDeleteClient{err: testDeleteAPIError{code: "NoSuchKey", msg: "missing"}}
			},
		}

		result, err := helper.DeleteObject(context.Background(), DeleteObjectInput{
			BucketName:      "bucket-a",
			ObjectKey:       "uploads/a.jpg",
			Region:          "us-east-1",
			RoleARN:         "arn:aws:iam::123456789012:role/s3",
			AllowedPrefixes: []string{"uploads/"},
		})
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}
		if !result.Deleted {
			t.Fatal("expected deleted=true")
		}
	})

	t.Run("returns upstream delete error", func(t *testing.T) {
		helper := &DeleteHelper{
			cache:       &stubDeleteRoleConfigProvider{cfg: aws.Config{}},
			retryPolicy: retryPolicy{maxAttempts: 2, sleep: func(context.Context, time.Duration) error { return nil }},
			clientFactory: func(aws.Config) deleteClient {
				return &stubDeleteClient{err: errors.New("s3 timeout")}
			},
		}

		_, err := helper.DeleteObject(context.Background(), DeleteObjectInput{
			BucketName:      "bucket-a",
			ObjectKey:       "uploads/a.jpg",
			Region:          "us-east-1",
			RoleARN:         "arn:aws:iam::123456789012:role/s3",
			AllowedPrefixes: []string{"uploads/"},
		})
		if err == nil {
			t.Fatal("expected error")
		}
		if !strings.Contains(err.Error(), "delete object") {
			t.Fatalf("expected wrapped delete error, got %v", err)
		}
	})

	t.Run("retries on transient throttling error", func(t *testing.T) {
		client := &stubDeleteClient{errs: []error{testDeleteAPIError{code: "SlowDown", msg: "retry"}, nil}}
		helper := &DeleteHelper{
			cache:       &stubDeleteRoleConfigProvider{cfg: aws.Config{}},
			retryPolicy: retryPolicy{maxAttempts: 3, sleep: func(context.Context, time.Duration) error { return nil }},
			clientFactory: func(aws.Config) deleteClient {
				return client
			},
		}

		result, err := helper.DeleteObject(context.Background(), DeleteObjectInput{
			BucketName:      "bucket-a",
			ObjectKey:       "uploads/a.jpg",
			Region:          "us-east-1",
			RoleARN:         "arn:aws:iam::123456789012:role/s3",
			AllowedPrefixes: []string{"uploads/"},
		})
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}
		if !result.Deleted {
			t.Fatal("expected deleted=true")
		}
		if client.calls != 2 {
			t.Fatalf("expected 2 delete attempts, got %d", client.calls)
		}
	})
}
