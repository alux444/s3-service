package s3

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	awss3 "github.com/aws/aws-sdk-go-v2/service/s3"
)

const defaultListMaxObjects = int32(200)

type ListObjectsInput struct {
	BucketName        string
	Prefix            string
	Region            string
	RoleARN           string
	ExternalID        *string
	ContinuationToken *string
}

type ListedObject struct {
	ObjectKey    string
	Size         int64
	ETag         string
	LastModified time.Time
}

type ListObjectsPageResult struct {
	Objects               []ListedObject
	NextContinuationToken *string
}

type listRoleConfigProvider interface {
	ConfigForRole(ctx context.Context, ref BucketRoleReference) (aws.Config, error)
}

type listClient interface {
	ListObjectsV2(ctx context.Context, params *awss3.ListObjectsV2Input, optFns ...func(*awss3.Options)) (*awss3.ListObjectsV2Output, error)
}

type listClientFactory func(cfg aws.Config) listClient

type ListHelperOption func(*ListHelper)

func WithListRetryPolicy(policy retryPolicy) ListHelperOption {
	return func(h *ListHelper) {
		h.retryPolicy = policy
	}
}

func WithListClientFactory(factory listClientFactory) ListHelperOption {
	return func(h *ListHelper) {
		if factory != nil {
			h.clientFactory = factory
		}
	}
}

func WithListMaxObjects(max int32) ListHelperOption {
	return func(h *ListHelper) {
		if max > 0 {
			h.maxObjects = max
		}
	}
}

func WithListLogger(logger *slog.Logger) ListHelperOption {
	return func(h *ListHelper) {
		h.logger = logger
	}
}

type ListHelper struct {
	cache         listRoleConfigProvider
	clientFactory listClientFactory
	retryPolicy   retryPolicy
	maxObjects    int32
	logger        *slog.Logger
}

func NewListHelper(cache *AssumeRoleSessionCache, opts ...ListHelperOption) *ListHelper {
	helper := &ListHelper{
		cache:       cache,
		retryPolicy: defaultRetryPolicy(),
		maxObjects:  defaultListMaxObjects,
		logger:      slog.Default(),
		clientFactory: func(cfg aws.Config) listClient {
			return awss3.NewFromConfig(cfg)
		},
	}

	for _, opt := range opts {
		opt(helper)
	}

	return helper
}

func (h *ListHelper) ListObjects(ctx context.Context, input ListObjectsInput) ([]ListedObject, error) {
	result, err := h.ListObjectsPage(ctx, input)
	if err != nil {
		return nil, err
	}
	return result.Objects, nil
}

func (h *ListHelper) ListObjectsPage(ctx context.Context, input ListObjectsInput) (ListObjectsPageResult, error) {
	if h == nil || h.cache == nil {
		return ListObjectsPageResult{}, errors.New("list helper is not configured")
	}
	if input.BucketName == "" || input.Region == "" || input.RoleARN == "" {
		return ListObjectsPageResult{}, fmt.Errorf("%w: bucket_name, region, and role_arn are required", ErrInvalidAssumeRoleInput)
	}

	cfg, err := h.cache.ConfigForRole(ctx, BucketRoleReference{Region: input.Region, RoleARN: input.RoleARN, ExternalID: input.ExternalID})
	if err != nil {
		return ListObjectsPageResult{}, fmt.Errorf("resolve role config for list objects: %w", err)
	}

	client := h.clientFactory(cfg)
	objects := make([]ListedObject, 0)
	continuationToken := input.ContinuationToken
	var nextToken *string

	for int32(len(objects)) < h.maxObjects {
		remaining := h.maxObjects - int32(len(objects))
		if remaining <= 0 {
			break
		}
		maxKeys := remaining
		if maxKeys > 1000 {
			maxKeys = 1000
		}

		out, err := retryAWS(ctx, h.retryPolicy, func() (*awss3.ListObjectsV2Output, error) {
			return client.ListObjectsV2(ctx, &awss3.ListObjectsV2Input{
				Bucket:            aws.String(input.BucketName),
				Prefix:            aws.String(input.Prefix),
				ContinuationToken: continuationToken,
				MaxKeys:           aws.Int32(maxKeys),
			})
		})
		if err != nil {
			return ListObjectsPageResult{}, fmt.Errorf("list objects: %w", err)
		}

		for _, item := range out.Contents {
			objects = append(objects, ListedObject{
				ObjectKey:    aws.ToString(item.Key),
				Size:         aws.ToInt64(item.Size),
				ETag:         aws.ToString(item.ETag),
				LastModified: aws.ToTime(item.LastModified),
			})
			if int32(len(objects)) >= h.maxObjects {
				if aws.ToBool(out.IsTruncated) && out.NextContinuationToken != nil {
					nextToken = out.NextContinuationToken
					if h.logger != nil {
						h.logger.InfoContext(ctx, "s3 list batch reached max objects; next continuation token available", "bucket_name", input.BucketName, "prefix", input.Prefix, "max_objects", h.maxObjects, "next_continuation_token", aws.ToString(nextToken))
					}
				}
				break
			}
		}

		if nextToken != nil {
			break
		}

		if !aws.ToBool(out.IsTruncated) || out.NextContinuationToken == nil {
			break
		}
		continuationToken = out.NextContinuationToken
	}

	return ListObjectsPageResult{Objects: objects, NextContinuationToken: nextToken}, nil
}
