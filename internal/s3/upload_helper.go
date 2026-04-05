package s3

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	awss3 "github.com/aws/aws-sdk-go-v2/service/s3"
)

var ErrUnsupportedContentType = errors.New("unsupported content type")
var ErrObjectTooLarge = errors.New("object exceeds max upload size")

const defaultMaxUploadBytes int64 = 20 * 1024 * 1024 // 20 MiB

var defaultAllowedContentTypes = []string{
	"image/jpeg",
	"image/png",
	"image/webp",
	"image/gif",
	"image/heic",
	"image/heif",
}

// UploadObjectInput captures all data needed for an S3 PutObject operation.
type UploadObjectInput struct {
	BucketName  string
	ObjectKey   string
	Region      string
	RoleARN     string
	ExternalID  *string
	ContentType string
	Body        []byte
	Metadata    map[string]string
}

// UploadObjectResult provides useful return info for API responses and audit logs.
type UploadObjectResult struct {
	ETag string
	Size int64
}

type uploadRoleConfigProvider interface {
	ConfigForRole(ctx context.Context, ref BucketRoleReference) (aws.Config, error)
}

type uploadClient interface {
	PutObject(ctx context.Context, params *awss3.PutObjectInput, optFns ...func(*awss3.Options)) (*awss3.PutObjectOutput, error)
}

type uploadClientFactory func(cfg aws.Config) uploadClient

type UploadHelperOption func(*UploadHelper)

func WithAllowedContentTypes(contentTypes []string) UploadHelperOption {
	return func(h *UploadHelper) {
		if len(contentTypes) == 0 {
			return
		}
		h.allowedContentTypes = make(map[string]struct{}, len(contentTypes))
		for _, c := range contentTypes {
			normalized := strings.ToLower(strings.TrimSpace(c))
			if normalized == "" {
				continue
			}
			h.allowedContentTypes[normalized] = struct{}{}
		}
	}
}

func WithMaxUploadBytes(maxBytes int64) UploadHelperOption {
	return func(h *UploadHelper) {
		if maxBytes > 0 {
			h.maxUploadBytes = maxBytes
		}
	}
}

func WithUploadClientFactory(factory uploadClientFactory) UploadHelperOption {
	return func(h *UploadHelper) {
		if factory != nil {
			h.clientFactory = factory
		}
	}
}

// UploadHelper owns low-level upload concerns for S3.
type UploadHelper struct {
	cache               uploadRoleConfigProvider
	allowedContentTypes map[string]struct{}
	maxUploadBytes      int64
	clientFactory       uploadClientFactory
}

func NewUploadHelper(cache *AssumeRoleSessionCache, opts ...UploadHelperOption) *UploadHelper {
	helper := &UploadHelper{
		cache:               cache,
		maxUploadBytes:      defaultMaxUploadBytes,
		allowedContentTypes: make(map[string]struct{}, len(defaultAllowedContentTypes)),
		clientFactory: func(cfg aws.Config) uploadClient {
			return awss3.NewFromConfig(cfg)
		},
	}

	for _, contentType := range defaultAllowedContentTypes {
		helper.allowedContentTypes[contentType] = struct{}{}
	}

	for _, opt := range opts {
		opt(helper)
	}

	return helper
}

func (h *UploadHelper) UploadObject(ctx context.Context, input UploadObjectInput) (UploadObjectResult, error) {
	if h == nil || h.cache == nil {
		return UploadObjectResult{}, errors.New("upload helper is not configured")
	}

	if input.BucketName == "" || input.ObjectKey == "" || input.Region == "" || input.RoleARN == "" {
		return UploadObjectResult{}, fmt.Errorf("%w: bucket_name, object_key, region, and role_arn are required", ErrInvalidAssumeRoleInput)
	}

	normalizedType := strings.ToLower(strings.TrimSpace(input.ContentType))
	if normalizedType == "" {
		return UploadObjectResult{}, fmt.Errorf("%w: content_type is required", ErrUnsupportedContentType)
	}
	if _, ok := h.allowedContentTypes[normalizedType]; !ok {
		return UploadObjectResult{}, fmt.Errorf("%w: %s", ErrUnsupportedContentType, input.ContentType)
	}

	if len(input.Body) == 0 {
		return UploadObjectResult{}, fmt.Errorf("%w: body is required", ErrInvalidAssumeRoleInput)
	}
	if int64(len(input.Body)) > h.maxUploadBytes {
		return UploadObjectResult{}, fmt.Errorf("%w: size=%d max=%d", ErrObjectTooLarge, len(input.Body), h.maxUploadBytes)
	}

	cfg, err := h.cache.ConfigForRole(ctx, BucketRoleReference{Region: input.Region, RoleARN: input.RoleARN, ExternalID: input.ExternalID})
	if err != nil {
		return UploadObjectResult{}, fmt.Errorf("resolve role config for upload: %w", err)
	}

	client := h.clientFactory(cfg)
	output, err := client.PutObject(ctx, &awss3.PutObjectInput{
		Bucket:      aws.String(input.BucketName),
		Key:         aws.String(input.ObjectKey),
		Body:        bytes.NewReader(input.Body),
		ContentType: aws.String(normalizedType),
		Metadata:    input.Metadata,
	})
	if err != nil {
		return UploadObjectResult{}, fmt.Errorf("put object: %w", err)
	}

	return UploadObjectResult{
		ETag: aws.ToString(output.ETag),
		Size: int64(len(input.Body)),
	}, nil
}
