package s3

import (
	"context"
	"errors"
	"fmt"
	"io"

	"github.com/aws/aws-sdk-go-v2/aws"
	awss3 "github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/smithy-go"
)

var ErrReadPrefixGuardrailViolation = errors.New("object key violates read prefix guardrails")
var ErrObjectNotFound = errors.New("object not found")

type GetObjectInput struct {
	BucketName      string
	ObjectKey       string
	Region          string
	RoleARN         string
	ExternalID      *string
	AllowedPrefixes []string
}

type GetObjectResult struct {
	Body          io.ReadCloser
	ContentType   string
	ContentLength int64
	ETag          string
}

type getRoleConfigProvider interface {
	ConfigForRole(ctx context.Context, ref BucketRoleReference) (aws.Config, error)
}

type getClient interface {
	GetObject(ctx context.Context, params *awss3.GetObjectInput, optFns ...func(*awss3.Options)) (*awss3.GetObjectOutput, error)
}

type getClientFactory func(cfg aws.Config) getClient

type GetHelperOption func(*GetHelper)

func WithGetRetryPolicy(policy retryPolicy) GetHelperOption {
	return func(h *GetHelper) {
		h.retryPolicy = policy
	}
}

func WithGetClientFactory(factory getClientFactory) GetHelperOption {
	return func(h *GetHelper) {
		if factory != nil {
			h.clientFactory = factory
		}
	}
}

type GetHelper struct {
	cache         getRoleConfigProvider
	clientFactory getClientFactory
	retryPolicy   retryPolicy
}

func NewGetHelper(cache *AssumeRoleSessionCache, opts ...GetHelperOption) *GetHelper {
	helper := &GetHelper{
		cache:       cache,
		retryPolicy: defaultRetryPolicy(),
		clientFactory: func(cfg aws.Config) getClient {
			return awss3.NewFromConfig(cfg)
		},
	}

	for _, opt := range opts {
		opt(helper)
	}

	return helper
}

func (h *GetHelper) GetObject(ctx context.Context, input GetObjectInput) (GetObjectResult, error) {
	if h == nil || h.cache == nil {
		return GetObjectResult{}, errors.New("get helper is not configured")
	}
	if input.BucketName == "" || input.ObjectKey == "" || input.Region == "" || input.RoleARN == "" {
		return GetObjectResult{}, fmt.Errorf("%w: bucket_name, object_key, region, and role_arn are required", ErrInvalidAssumeRoleInput)
	}
	if !isWithinAllowedPrefixes(input.ObjectKey, input.AllowedPrefixes) {
		return GetObjectResult{}, fmt.Errorf("%w: object key is outside allowed prefixes", ErrReadPrefixGuardrailViolation)
	}

	cfg, err := h.cache.ConfigForRole(ctx, BucketRoleReference{Region: input.Region, RoleARN: input.RoleARN, ExternalID: input.ExternalID})
	if err != nil {
		return GetObjectResult{}, fmt.Errorf("resolve role config for get object: %w", err)
	}

	client := h.clientFactory(cfg)
	output, err := retryAWS(ctx, h.retryPolicy, func() (*awss3.GetObjectOutput, error) {
		return client.GetObject(ctx, &awss3.GetObjectInput{
			Bucket: aws.String(input.BucketName),
			Key:    aws.String(input.ObjectKey),
		})
	})
	if err != nil {
		if isNoSuchObjectLike(err) {
			return GetObjectResult{}, fmt.Errorf("%w: %s", ErrObjectNotFound, input.ObjectKey)
		}
		return GetObjectResult{}, fmt.Errorf("get object: %w", err)
	}
	if output.Body == nil {
		return GetObjectResult{}, errors.New("get object returned empty body")
	}

	return GetObjectResult{
		Body:          output.Body,
		ContentType:   aws.ToString(output.ContentType),
		ContentLength: aws.ToInt64(output.ContentLength),
		ETag:          aws.ToString(output.ETag),
	}, nil
}

func isNoSuchObjectLike(err error) bool {
	var apiErr smithy.APIError
	if !errors.As(err, &apiErr) {
		return false
	}
	code := apiErr.ErrorCode()
	return code == "NoSuchKey" || code == "NotFound"
}
