package s3

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
)

type testPresignProvider struct {
	value aws.Credentials
	err   error
}

func (p testPresignProvider) Retrieve(context.Context) (aws.Credentials, error) {
	if p.err != nil {
		return aws.Credentials{}, p.err
	}
	return p.value, nil
}

func newTestPresignCache(t *testing.T) *AssumeRoleSessionCache {
	t.Helper()

	cache, err := NewAssumeRoleSessionCache(
		context.Background(),
		aws.Config{},
		WithProviderFactory(func(_ BucketRoleReference, _ time.Duration) (aws.CredentialsProvider, error) {
			return testPresignProvider{value: aws.Credentials{
				AccessKeyID:     "AKIA_TEST",
				SecretAccessKey: "SECRET_TEST",
				SessionToken:    "TOKEN_TEST",
				CanExpire:       true,
				Expires:         time.Now().Add(10 * time.Minute),
			}}, nil
		}),
	)
	if err != nil {
		t.Fatalf("NewAssumeRoleSessionCache() error = %v", err)
	}

	return cache
}

func TestPresignHelper_PresignObject(t *testing.T) {
	t.Run("returns validation error for missing required fields", func(t *testing.T) {
		helper := NewPresignHelper(newTestPresignCache(t))

		_, err := helper.PresignObject(context.Background(), PresignObjectInput{Method: "GET"})
		if err == nil {
			t.Fatal("expected error, got nil")
		}
		if !errors.Is(err, ErrInvalidAssumeRoleInput) {
			t.Fatalf("expected ErrInvalidAssumeRoleInput, got %v", err)
		}
	})

	t.Run("returns unsupported method error", func(t *testing.T) {
		helper := NewPresignHelper(newTestPresignCache(t))

		_, err := helper.PresignObject(context.Background(), PresignObjectInput{
			BucketName: "bucket-a",
			ObjectKey:  "uploads/a.jpg",
			Region:     "us-east-1",
			RoleARN:    "arn:aws:iam::123456789012:role/test",
			Method:     "DELETE",
		})
		if err == nil {
			t.Fatal("expected error, got nil")
		}
		if !errors.Is(err, ErrUnsupportedPresignMethod) {
			t.Fatalf("expected ErrUnsupportedPresignMethod, got %v", err)
		}
	})

	t.Run("presigns get object", func(t *testing.T) {
		helper := NewPresignHelper(newTestPresignCache(t))

		result, err := helper.PresignObject(context.Background(), PresignObjectInput{
			BucketName: "bucket-a",
			ObjectKey:  "uploads/a.jpg",
			Region:     "us-east-1",
			RoleARN:    "arn:aws:iam::123456789012:role/test",
			Method:     "GET",
			ExpiresIn:  90 * time.Second,
		})
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}
		if result.Method != "GET" {
			t.Fatalf("expected method GET, got %q", result.Method)
		}
		if result.URL == "" {
			t.Fatal("expected non-empty presigned URL")
		}
		if !strings.Contains(result.URL, "X-Amz-Signature=") {
			t.Fatalf("expected signed URL, got %q", result.URL)
		}
		if result.ExpiresIn != 90*time.Second {
			t.Fatalf("expected ExpiresIn=90s, got %s", result.ExpiresIn)
		}
	})

	t.Run("presigns put object", func(t *testing.T) {
		helper := NewPresignHelper(newTestPresignCache(t))

		result, err := helper.PresignObject(context.Background(), PresignObjectInput{
			BucketName:  "bucket-a",
			ObjectKey:   "uploads/a.jpg",
			Region:      "us-east-1",
			RoleARN:     "arn:aws:iam::123456789012:role/test",
			Method:      "PUT",
			ExpiresIn:   120 * time.Second,
			ContentType: "image/jpeg",
		})
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}
		if result.Method != "PUT" {
			t.Fatalf("expected method PUT, got %q", result.Method)
		}
		if result.URL == "" {
			t.Fatal("expected non-empty presigned URL")
		}
		if !strings.Contains(result.URL, "X-Amz-Signature=") {
			t.Fatalf("expected signed URL, got %q", result.URL)
		}
		if result.ExpiresIn != 120*time.Second {
			t.Fatalf("expected ExpiresIn=120s, got %s", result.ExpiresIn)
		}
	})
}
