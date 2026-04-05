package s3

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	awss3 "github.com/aws/aws-sdk-go-v2/service/s3"
)

var ErrPresignNotImplemented = errors.New("s3 presign helper not implemented")
var ErrUnsupportedPresignMethod = errors.New("unsupported presign method")

type PresignObjectInput struct {
	BucketName  string
	ObjectKey   string
	Region      string
	RoleARN     string
	ExternalID  *string
	Method      string // expected: GET or PUT
	ExpiresIn   time.Duration
	ContentType string
}

type PresignObjectResult struct {
	URL       string
	ExpiresIn time.Duration
	Method    string
}

// PresignHelper owns low-level S3 presign URL generation.
//
// WISHFUL THINKING (3.6):
// - Build role-scoped S3 client via AssumeRoleSessionCache.
// - Use AWS s3/presign client for GET and PUT.
// - Enforce short expiration defaults and hard max limits.
// - Include signed headers/content type constraints for PUT URLs.
// - Return typed validation errors for unsupported methods/expiry.
type PresignHelper struct {
	cache *AssumeRoleSessionCache
}

func NewPresignHelper(cache *AssumeRoleSessionCache) *PresignHelper {
	return &PresignHelper{cache: cache}
}

func (h *PresignHelper) PresignObject(ctx context.Context, input PresignObjectInput) (PresignObjectResult, error) {
	if input.BucketName == "" || input.ObjectKey == "" || input.Region == "" || input.RoleARN == "" {
		return PresignObjectResult{}, fmt.Errorf("%w: bucket_name, object_key, region, and role_arn are required", ErrInvalidAssumeRoleInput)
	}
	if h == nil || h.cache == nil {
		return PresignObjectResult{}, errors.New("presign helper is not configured")
	}
	method := strings.ToUpper(strings.TrimSpace(input.Method))
	if method == "" {
		return PresignObjectResult{}, fmt.Errorf("%w: method is required", ErrInvalidAssumeRoleInput)
	}
	if method != "GET" && method != "PUT" {
		return PresignObjectResult{}, fmt.Errorf("%w: %s", ErrUnsupportedPresignMethod, method)
	}

	if method == "GET" {
		cfg, err := h.cache.ConfigForRole(ctx, BucketRoleReference{
			Region:     input.Region,
			RoleARN:    input.RoleARN,
			ExternalID: input.ExternalID,
		})
		if err != nil {
			return PresignObjectResult{}, fmt.Errorf("resolve role config for presign: %w", err)
		}

		presigner := awss3.NewPresignClient(awss3.NewFromConfig(cfg))
		request, err := presigner.PresignGetObject(ctx, &awss3.GetObjectInput{
			Bucket: aws.String(input.BucketName),
			Key:    aws.String(input.ObjectKey),
		}, func(options *awss3.PresignOptions) {
			if input.ExpiresIn > 0 {
				options.Expires = input.ExpiresIn
			}
		})
		if err != nil {
			return PresignObjectResult{}, fmt.Errorf("presign get object: %w", err)
		}

		result := PresignObjectResult{
			URL:    request.URL,
			Method: method,
		}
		if input.ExpiresIn > 0 {
			result.ExpiresIn = input.ExpiresIn
		}
		return result, nil
	}

	if method == "PUT" {
		cfg, err := h.cache.ConfigForRole(ctx, BucketRoleReference{
			Region:     input.Region,
			RoleARN:    input.RoleARN,
			ExternalID: input.ExternalID,
		})
		if err != nil {
			return PresignObjectResult{}, fmt.Errorf("resolve role config for presign: %w", err)
		}

		presigner := awss3.NewPresignClient(awss3.NewFromConfig(cfg))
		putInput := &awss3.PutObjectInput{
			Bucket: aws.String(input.BucketName),
			Key:    aws.String(input.ObjectKey),
		}
		if input.ContentType != "" {
			putInput.ContentType = aws.String(input.ContentType)
		}

		request, err := presigner.PresignPutObject(ctx, putInput, func(options *awss3.PresignOptions) {
			if input.ExpiresIn > 0 {
				options.Expires = input.ExpiresIn
			}
		})
		if err != nil {
			return PresignObjectResult{}, fmt.Errorf("presign put object: %w", err)
		}

		result := PresignObjectResult{
			URL:    request.URL,
			Method: method,
		}
		if input.ExpiresIn > 0 {
			result.ExpiresIn = input.ExpiresIn
		}
		return result, nil
	}

	return PresignObjectResult{}, ErrPresignNotImplemented
}
