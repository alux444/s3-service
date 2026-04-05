package service

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"
)

var ErrInvalidObjectPresignInput = errors.New("invalid object presign input")
var ErrObjectPresignNotImplemented = errors.New("object presign service not implemented")

const (
	PresignMethodGet = "GET"
	PresignMethodPut = "PUT"
)

const (
	MinPresignTTL        = 30 * time.Second
	DefaultPresignPutTTL = 60 * time.Second
	DefaultPresignGetTTL = 120 * time.Second
	MaxPresignPutTTL     = 5 * time.Minute
	MaxPresignGetTTL     = 10 * time.Minute
)

type ObjectPresigner interface {
	PresignObject(ctx context.Context, input ObjectPresignInput) (ObjectPresignResult, error)
}

type ObjectPresignInput struct {
	BucketName  string
	ObjectKey   string
	ProjectID   string
	AppID       string
	Region      string
	RoleARN     string
	ExternalID  *string
	Method      string
	ExpiresIn   time.Duration
	ContentType string
}

type ObjectPresignResult struct {
	URL       string
	Method    string
	ExpiresIn time.Duration
}

// ObjectPresignService orchestrates business-level presign flow.
//
// WISHFUL THINKING (3.6):
// - Resolve bucket connection metadata by project/app scope (same pattern as upload/delete).
// - Enforce method-level policy rules (GET vs PUT) before delegating.
// - Enforce short expiration defaults and max bounds in service layer.
// - Carry content type constraints into PUT presign inputs.
type ObjectPresignService struct {
	bucketRepo ObjectUploadBucketRepository
	presigner  ObjectPresigner
}

func NewObjectPresignService(bucketRepo ObjectUploadBucketRepository, presigner ObjectPresigner) *ObjectPresignService {
	return &ObjectPresignService{bucketRepo: bucketRepo, presigner: presigner}
}

func (s *ObjectPresignService) PresignObject(ctx context.Context, input ObjectPresignInput) (ObjectPresignResult, error) {
	if input.ProjectID == "" || input.AppID == "" || input.BucketName == "" || input.ObjectKey == "" || input.Method == "" {
		return ObjectPresignResult{}, fmt.Errorf("%w: project_id, app_id, bucket_name, object_key, and method are required", ErrInvalidObjectPresignInput)
	}
	if s.bucketRepo == nil {
		return ObjectPresignResult{}, errors.New("object presign bucket repository dependency is not configured")
	}
	if s.presigner == nil {
		return ObjectPresignResult{}, errors.New("object presigner dependency is not configured")
	}

	// WISHFUL THINKING (3.6):
	// 1) List bucket connections for scope and resolve selected bucket metadata
	// 2) Return ErrBucketConnectionNotFound when no scope match exists
	// 3) Apply method-specific defaults for ExpiresIn (short-lived)
	// 4) Delegate to presigner and return normalized output
	return ObjectPresignResult{}, ErrObjectPresignNotImplemented
}

func normalizePresignTTL(method string, requested time.Duration) time.Duration {
	method = strings.ToUpper(strings.TrimSpace(method))

	defaultTTL := DefaultPresignGetTTL
	maxTTL := MaxPresignGetTTL
	if method == PresignMethodPut {
		defaultTTL = DefaultPresignPutTTL
		maxTTL = MaxPresignPutTTL
	}

	if requested <= 0 {
		return defaultTTL
	}
	if requested < MinPresignTTL {
		return MinPresignTTL
	}
	if requested > maxTTL {
		return maxTTL
	}

	return requested
}
