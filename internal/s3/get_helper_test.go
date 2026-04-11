package s3

import (
	"context"
	"errors"
	"io"
	"strings"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	awss3 "github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/smithy-go"
)

type stubGetRoleConfigProvider struct {
	cfg aws.Config
	err error
	ref BucketRoleReference
}

func (s *stubGetRoleConfigProvider) ConfigForRole(_ context.Context, ref BucketRoleReference) (aws.Config, error) {
	s.ref = ref
	if s.err != nil {
		return aws.Config{}, s.err
	}
	return s.cfg, nil
}

type stubGetClient struct {
	out   *awss3.GetObjectOutput
	err   error
	errs  []error
	input *awss3.GetObjectInput
	calls int
}

func (s *stubGetClient) GetObject(_ context.Context, params *awss3.GetObjectInput, _ ...func(*awss3.Options)) (*awss3.GetObjectOutput, error) {
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
	if s.out == nil {
		return &awss3.GetObjectOutput{}, nil
	}
	return s.out, nil
}

type testGetAPIError struct {
	code string
	msg  string
}

func (e testGetAPIError) ErrorCode() string             { return e.code }
func (e testGetAPIError) ErrorMessage() string          { return e.msg }
func (e testGetAPIError) ErrorFault() smithy.ErrorFault { return smithy.FaultClient }
func (e testGetAPIError) Error() string                 { return e.code + ": " + e.msg }

func TestGetHelper_GetObject(t *testing.T) {
	t.Run("reads object when input is valid", func(t *testing.T) {
		provider := &stubGetRoleConfigProvider{cfg: aws.Config{Region: "us-east-1"}}
		client := &stubGetClient{out: &awss3.GetObjectOutput{Body: io.NopCloser(strings.NewReader("payload")), ContentType: aws.String("image/jpeg"), ContentLength: aws.Int64(7), ETag: aws.String("\"etag-1\"")}}
		helper := &GetHelper{
			cache: provider,
			clientFactory: func(aws.Config) getClient {
				return client
			},
		}

		externalID := "ext-1"
		result, err := helper.GetObject(context.Background(), GetObjectInput{
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
		if provider.ref.Region != "us-east-1" || provider.ref.RoleARN == "" {
			t.Fatalf("unexpected role ref: %+v", provider.ref)
		}
		if client.input == nil || aws.ToString(client.input.Key) != "uploads/a.jpg" {
			t.Fatalf("unexpected get input: %+v", client.input)
		}
		if result.ContentType != "image/jpeg" || result.ContentLength != 7 || result.ETag != "\"etag-1\"" {
			t.Fatalf("unexpected result metadata: %+v", result)
		}
		body, readErr := io.ReadAll(result.Body)
		if readErr != nil {
			t.Fatalf("failed reading body: %v", readErr)
		}
		if string(body) != "payload" {
			t.Fatalf("unexpected body: %q", string(body))
		}
	})

	t.Run("returns guardrail error when key is outside allowed prefixes", func(t *testing.T) {
		helper := &GetHelper{cache: &stubGetRoleConfigProvider{cfg: aws.Config{}}, clientFactory: func(aws.Config) getClient { return &stubGetClient{} }}

		_, err := helper.GetObject(context.Background(), GetObjectInput{
			BucketName:      "bucket-a",
			ObjectKey:       "private/a.jpg",
			Region:          "us-east-1",
			RoleARN:         "arn:aws:iam::123456789012:role/s3",
			AllowedPrefixes: []string{"uploads/"},
		})
		if err == nil {
			t.Fatal("expected error")
		}
		if !errors.Is(err, ErrReadPrefixGuardrailViolation) {
			t.Fatalf("expected ErrReadPrefixGuardrailViolation, got %v", err)
		}
	})

	t.Run("maps missing object to ErrObjectNotFound", func(t *testing.T) {
		helper := &GetHelper{
			cache: &stubGetRoleConfigProvider{cfg: aws.Config{}},
			clientFactory: func(aws.Config) getClient {
				return &stubGetClient{err: testGetAPIError{code: "NoSuchKey", msg: "missing"}}
			},
		}

		_, err := helper.GetObject(context.Background(), GetObjectInput{
			BucketName:      "bucket-a",
			ObjectKey:       "uploads/a.jpg",
			Region:          "us-east-1",
			RoleARN:         "arn:aws:iam::123456789012:role/s3",
			AllowedPrefixes: []string{"uploads/"},
		})
		if err == nil {
			t.Fatal("expected error")
		}
		if !errors.Is(err, ErrObjectNotFound) {
			t.Fatalf("expected ErrObjectNotFound, got %v", err)
		}
	})

	t.Run("retries on transient throttling error", func(t *testing.T) {
		client := &stubGetClient{errs: []error{testGetAPIError{code: "SlowDown", msg: "retry"}, nil}, out: &awss3.GetObjectOutput{Body: io.NopCloser(strings.NewReader("payload"))}}
		helper := &GetHelper{
			cache:       &stubGetRoleConfigProvider{cfg: aws.Config{}},
			retryPolicy: retryPolicy{maxAttempts: 3, sleep: func(context.Context, time.Duration) error { return nil }},
			clientFactory: func(aws.Config) getClient {
				return client
			},
		}

		result, err := helper.GetObject(context.Background(), GetObjectInput{
			BucketName:      "bucket-a",
			ObjectKey:       "uploads/a.jpg",
			Region:          "us-east-1",
			RoleARN:         "arn:aws:iam::123456789012:role/s3",
			AllowedPrefixes: []string{"uploads/"},
		})
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}
		if result.Body == nil {
			t.Fatal("expected body")
		}
		if client.calls != 2 {
			t.Fatalf("expected 2 get attempts, got %d", client.calls)
		}
	})
}
