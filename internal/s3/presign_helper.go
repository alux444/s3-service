package s3

import (
	"context"
	"errors"
	"fmt"
	"time"
)

var ErrPresignNotImplemented = errors.New("s3 presign helper not implemented")

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
	if input.Method == "" {
		return PresignObjectResult{}, fmt.Errorf("%w: method is required", ErrInvalidAssumeRoleInput)
	}

	// WISHFUL THINKING (3.6):
	// 1) Resolve assumed-role config from h.cache.ConfigForRole(...)
	// 2) Build S3 presign client from role-scoped config
	// 3) Normalize and validate expiration window (short-lived)
	// 4) Branch by method: PresignGetObject / PresignPutObject
	// 5) Return URL + method + final expiration
	return PresignObjectResult{}, ErrPresignNotImplemented
}
