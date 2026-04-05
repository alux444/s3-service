package s3

import (
	"context"
	"errors"
	"io"
	"strings"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	awss3 "github.com/aws/aws-sdk-go-v2/service/s3"
)

type stubUploadRoleConfigProvider struct {
	cfg aws.Config
	err error
	ref BucketRoleReference
}

func (s *stubUploadRoleConfigProvider) ConfigForRole(_ context.Context, ref BucketRoleReference) (aws.Config, error) {
	s.ref = ref
	if s.err != nil {
		return aws.Config{}, s.err
	}
	return s.cfg, nil
}

type stubUploadClient struct {
	out   *awss3.PutObjectOutput
	err   error
	input *awss3.PutObjectInput
}

func (s *stubUploadClient) PutObject(_ context.Context, params *awss3.PutObjectInput, _ ...func(*awss3.Options)) (*awss3.PutObjectOutput, error) {
	s.input = params
	if s.err != nil {
		return nil, s.err
	}
	if s.out == nil {
		return &awss3.PutObjectOutput{}, nil
	}
	return s.out, nil
}

func TestUploadHelper_UploadObject(t *testing.T) {
	t.Run("uploads with assumed role config and metadata", func(t *testing.T) {
		provider := &stubUploadRoleConfigProvider{cfg: aws.Config{Region: "us-east-1"}}
		client := &stubUploadClient{out: &awss3.PutObjectOutput{ETag: aws.String("\"etag-1\"")}}

		helper := &UploadHelper{
			cache: provider,
			allowedContentTypes: map[string]struct{}{
				"image/jpeg": {},
			},
			maxUploadBytes: 1024,
			clientFactory: func(aws.Config) uploadClient {
				return client
			},
		}

		externalID := "ext-1"
		result, err := helper.UploadObject(context.Background(), UploadObjectInput{
			BucketName:  "bucket-a",
			ObjectKey:   "uploads/a.jpg",
			Region:      "us-east-1",
			RoleARN:     "arn:aws:iam::123456789012:role/s3",
			ExternalID:  &externalID,
			ContentType: "image/jpeg",
			Body:        []byte("payload"),
			Metadata: map[string]string{
				"owner": "user-1",
			},
		})
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}
		if result.ETag != "\"etag-1\"" || result.Size != 7 {
			t.Fatalf("unexpected result: %+v", result)
		}
		if provider.ref.Region != "us-east-1" || provider.ref.RoleARN == "" {
			t.Fatalf("unexpected role ref: %+v", provider.ref)
		}
		if client.input == nil {
			t.Fatal("expected put object to be called")
		}
		if aws.ToString(client.input.Bucket) != "bucket-a" || aws.ToString(client.input.Key) != "uploads/a.jpg" {
			t.Fatalf("unexpected put object target: bucket=%s key=%s", aws.ToString(client.input.Bucket), aws.ToString(client.input.Key))
		}
		if aws.ToString(client.input.ContentType) != "image/jpeg" {
			t.Fatalf("expected content type image/jpeg, got %s", aws.ToString(client.input.ContentType))
		}
		if client.input.Metadata["owner"] != "user-1" {
			t.Fatalf("unexpected metadata: %+v", client.input.Metadata)
		}
		body, readErr := io.ReadAll(client.input.Body)
		if readErr != nil {
			t.Fatalf("failed reading put body: %v", readErr)
		}
		if string(body) != "payload" {
			t.Fatalf("unexpected body: %s", string(body))
		}
	})

	t.Run("rejects unsupported content type", func(t *testing.T) {
		helper := &UploadHelper{
			cache:               &stubUploadRoleConfigProvider{cfg: aws.Config{}},
			allowedContentTypes: map[string]struct{}{"image/png": {}},
			maxUploadBytes:      1024,
			clientFactory:       func(aws.Config) uploadClient { return &stubUploadClient{} },
		}

		_, err := helper.UploadObject(context.Background(), UploadObjectInput{
			BucketName:  "bucket-a",
			ObjectKey:   "uploads/a.jpg",
			Region:      "us-east-1",
			RoleARN:     "arn:aws:iam::123456789012:role/s3",
			ContentType: "application/pdf",
			Body:        []byte("payload"),
		})
		if err == nil {
			t.Fatal("expected error, got nil")
		}
		if !errors.Is(err, ErrUnsupportedContentType) {
			t.Fatalf("expected ErrUnsupportedContentType, got %v", err)
		}
	})

	t.Run("rejects payloads larger than max", func(t *testing.T) {
		helper := &UploadHelper{
			cache:               &stubUploadRoleConfigProvider{cfg: aws.Config{}},
			allowedContentTypes: map[string]struct{}{"image/png": {}},
			maxUploadBytes:      4,
			clientFactory:       func(aws.Config) uploadClient { return &stubUploadClient{} },
		}

		_, err := helper.UploadObject(context.Background(), UploadObjectInput{
			BucketName:  "bucket-a",
			ObjectKey:   "uploads/a.png",
			Region:      "us-east-1",
			RoleARN:     "arn:aws:iam::123456789012:role/s3",
			ContentType: "image/png",
			Body:        []byte("payload"),
		})
		if err == nil {
			t.Fatal("expected error, got nil")
		}
		if !errors.Is(err, ErrObjectTooLarge) {
			t.Fatalf("expected ErrObjectTooLarge, got %v", err)
		}
	})

	t.Run("returns upstream put object error", func(t *testing.T) {
		helper := &UploadHelper{
			cache:               &stubUploadRoleConfigProvider{cfg: aws.Config{}},
			allowedContentTypes: map[string]struct{}{"image/png": {}},
			maxUploadBytes:      1024,
			clientFactory: func(aws.Config) uploadClient {
				return &stubUploadClient{err: errors.New("s3 unavailable")}
			},
		}

		_, err := helper.UploadObject(context.Background(), UploadObjectInput{
			BucketName:  "bucket-a",
			ObjectKey:   "uploads/a.png",
			Region:      "us-east-1",
			RoleARN:     "arn:aws:iam::123456789012:role/s3",
			ContentType: "image/png",
			Body:        []byte("payload"),
		})
		if err == nil {
			t.Fatal("expected error, got nil")
		}
		if !strings.Contains(err.Error(), "put object") {
			t.Fatalf("expected put object wrapped error, got %v", err)
		}
	})
}
