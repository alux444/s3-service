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

	method := strings.ToUpper(strings.TrimSpace(input.Method))
	if method != PresignMethodGet && method != PresignMethodPut {
		return ObjectPresignResult{}, fmt.Errorf("%w: method must be GET or PUT", ErrInvalidObjectPresignInput)
	}
	if method == PresignMethodPut && strings.TrimSpace(input.ContentType) == "" {
		return ObjectPresignResult{}, fmt.Errorf("%w: content_type is required for PUT presign", ErrInvalidObjectPresignInput)
	}

	if s.bucketRepo == nil {
		return ObjectPresignResult{}, errors.New("object presign bucket repository dependency is not configured")
	}
	if s.presigner == nil {
		return ObjectPresignResult{}, errors.New("object presigner dependency is not configured")
	}

	buckets, err := s.bucketRepo.ListActiveBucketsForConnectionScope(ctx, input.ProjectID, input.AppID)
	if err != nil {
		return ObjectPresignResult{}, fmt.Errorf("list bucket connections for presign: %w", err)
	}

	var selectedBucket ObjectPresignInput
	found := false
	for i := range buckets {
		if buckets[i].BucketName == input.BucketName {
			selectedBucket = ObjectPresignInput{
				BucketName: input.BucketName,
				ObjectKey:  input.ObjectKey,
				ProjectID:  input.ProjectID,
				AppID:      input.AppID,
				Region:     buckets[i].Region,
				RoleARN:    buckets[i].RoleARN,
				ExternalID: buckets[i].ExternalID,
				Method:     method,
				ExpiresIn:  normalizePresignTTL(method, input.ExpiresIn),
			}
			if method == PresignMethodPut {
				selectedBucket.ContentType = input.ContentType
			}
			found = true
			break
		}
	}

	if !found {
		return ObjectPresignResult{}, fmt.Errorf("%w: %s", ErrBucketConnectionNotFound, input.BucketName)
	}

	result, err := s.presigner.PresignObject(ctx, selectedBucket)
	if err != nil {
		return ObjectPresignResult{}, err
	}

	return result, nil
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
