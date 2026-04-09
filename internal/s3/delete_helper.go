package s3

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	awss3 "github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/smithy-go"
)

var ErrDeletePrefixGuardrailViolation = errors.New("object key violates delete prefix guardrails")

type DeleteObjectInput struct {
	BucketName      string
	ObjectKey       string
	Region          string
	RoleARN         string
	ExternalID      *string
	AllowedPrefixes []string
}

type DeleteObjectResult struct {
	Deleted bool
}

type deleteRoleConfigProvider interface {
	ConfigForRole(ctx context.Context, ref BucketRoleReference) (aws.Config, error)
}

type deleteClient interface {
	DeleteObject(ctx context.Context, params *awss3.DeleteObjectInput, optFns ...func(*awss3.Options)) (*awss3.DeleteObjectOutput, error)
}

type deleteClientFactory func(cfg aws.Config) deleteClient

type DeleteHelperOption func(*DeleteHelper)

func WithDeleteRetryPolicy(policy retryPolicy) DeleteHelperOption {
	return func(h *DeleteHelper) {
		h.retryPolicy = policy
	}
}

func WithDeleteClientFactory(factory deleteClientFactory) DeleteHelperOption {
	return func(h *DeleteHelper) {
		if factory != nil {
			h.clientFactory = factory
		}
	}
}

type DeleteHelper struct {
	cache         deleteRoleConfigProvider
	clientFactory deleteClientFactory
	retryPolicy   retryPolicy
}

func NewDeleteHelper(cache *AssumeRoleSessionCache, opts ...DeleteHelperOption) *DeleteHelper {
	helper := &DeleteHelper{
		cache:       cache,
		retryPolicy: defaultRetryPolicy(),
		clientFactory: func(cfg aws.Config) deleteClient {
			return awss3.NewFromConfig(cfg)
		},
	}

	for _, opt := range opts {
		opt(helper)
	}

	return helper
}

func (h *DeleteHelper) DeleteObject(ctx context.Context, input DeleteObjectInput) (DeleteObjectResult, error) {
	if h == nil || h.cache == nil {
		return DeleteObjectResult{}, errors.New("delete helper is not configured")
	}
	if input.BucketName == "" || input.ObjectKey == "" || input.Region == "" || input.RoleARN == "" {
		return DeleteObjectResult{}, fmt.Errorf("%w: bucket_name, object_key, region, and role_arn are required", ErrInvalidAssumeRoleInput)
	}
	if !isDeleteKeySafe(input.ObjectKey) {
		return DeleteObjectResult{}, fmt.Errorf("%w: object key must target a concrete object", ErrDeletePrefixGuardrailViolation)
	}
	if !isWithinAllowedPrefixes(input.ObjectKey, input.AllowedPrefixes) {
		return DeleteObjectResult{}, fmt.Errorf("%w: object key is outside allowed prefixes", ErrDeletePrefixGuardrailViolation)
	}

	cfg, err := h.cache.ConfigForRole(ctx, BucketRoleReference{Region: input.Region, RoleARN: input.RoleARN, ExternalID: input.ExternalID})
	if err != nil {
		return DeleteObjectResult{}, fmt.Errorf("resolve role config for delete: %w", err)
	}

	client := h.clientFactory(cfg)
	_, err = retryAWS(ctx, h.retryPolicy, func() (*awss3.DeleteObjectOutput, error) {
		return client.DeleteObject(ctx, &awss3.DeleteObjectInput{
			Bucket: aws.String(input.BucketName),
			Key:    aws.String(input.ObjectKey),
		})
	})
	if err != nil {
		if isNoSuchKeyLike(err) {
			// Idempotent soft-fail: deleting a missing object is still success.
			return DeleteObjectResult{Deleted: true}, nil
		}
		return DeleteObjectResult{}, fmt.Errorf("delete object: %w", err)
	}

	return DeleteObjectResult{Deleted: true}, nil
}

func isDeleteKeySafe(objectKey string) bool {
	trimmed := strings.TrimSpace(objectKey)
	if trimmed == "" {
		return false
	}
	if strings.HasSuffix(trimmed, "/") {
		return false
	}
	if strings.Contains(trimmed, "*") {
		return false
	}
	return true
}

func isWithinAllowedPrefixes(objectKey string, prefixes []string) bool {
	if len(prefixes) == 0 {
		return false
	}
	for _, prefix := range prefixes {
		if prefix != "" && strings.HasPrefix(objectKey, prefix) {
			return true
		}
	}
	return false
}

func isNoSuchKeyLike(err error) bool {
	var apiErr smithy.APIError
	if !errors.As(err, &apiErr) {
		return false
	}
	code := apiErr.ErrorCode()
	return code == "NoSuchKey" || code == "NotFound"
}
